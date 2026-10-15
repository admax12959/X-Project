# 调研：从三个固定角色扩展到组织架构 + 角色权限（IAM）

日期：2026-10-14。术语以 `GLOSSARY.md` 为准；文中“部门”“授权”“权限点”“数据范围”是**建议新增**的术语，尚未写入词汇表。

## 现状（v1）

- v1 设计的角色只有三个：全局的 **管理员**，以及按 **应用** 授予的 **Owner** 和 **PE**，都是固定角色。
- 用户上有一个可选的“部门”字符串。绑定了 LDAP 的用户以 LDAP 为准，登录时同步（v1-scope、ADR-0002）。
- 约束：应用至少一个 Owner；不能禁用唯一 Owner（ADR-0003）；LDAP 不自动建用户也不自动合并（ADR-0002/0004）；内部关联一律用用户 ID。
- v1 范围规划了应用详情页的“变更历史”，记录 Owner/PE 的改动和实际操作人。

## 1. 组织模型

### 1.1 部门树怎么存（MySQL 8）

| 方案 | 读子树 | 移动子树 | 复杂度 |
|---|---|---|---|
| 邻接表 `parent_id` + `WITH RECURSIVE` | 递归 CTE，一条 SQL | 改一行 | 最低 |
| 物化路径 `path='/1/7/42/'` | `LIKE '/1/7/%'`，可以走索引 | 要改整棵子树的 path | 低 |
| 闭包表 `dept_closure(ancestor, descendant, depth)` | JOIN，最快 | 删旧路径再插新路径，O(子树×祖先) | 中 |
| 嵌套集 `lft/rgt` | 区间查询 | 要重排大半张表 | 高，不推荐 |

- MySQL 8 支持 `WITH RECURSIVE`。递归深度上限 `cte_max_recursion_depth` 默认 1000 层。公司部门树一般只有几百到几千个节点、深度不超过 10，这个上限碰不到。
- **建议**：用邻接表 + 递归 CTE。等“某部门及下级的所有应用”这类查询真的成了热点，再加一列物化路径或一张闭包表作为派生数据，在同一事务里维护。嵌套集不考虑。

### 1.2 用户和部门的关系

- 业界常见的做法是“一个主部门 + 若干附加归属”。Casdoor 的群组分 Physical 和 Virtual 两类：一个用户只能在一个 Physical 群组里，可以在任意多个 Virtual 群组里。Keycloak 和 authentik 的用户可以属于多个群组，而且子群组的成员算作父群组的成员（authentik 的原话是：“Members of children groups are effective members of the parent group”）。
- **建议**：`user_department(user_id, dept_id, is_primary)`，每个用户最多一个主部门。v1 的“部门”字符串**不再当作数据**：迁移时按名称匹配到部门节点并设为主部门；匹配不上的保留原值，作为只读的展示字段，等管理员处理。
- **部门负责人**：做成部门上的一个属性，`dept_leader(dept_id, user_id)`，可以有多个人，不必做成角色。它以后可以用于审批或“负责人能看到本部门的所有应用”，但下一期先只做展示。

### 1.3 部门树以 LDAP 为准，还是本地维护，还是两者都有

- **GitLab**：LDAP 群组同步只更新已有用户，不会新建用户。开启群组同步后，手工加入的非 LDAP 成员会被移除，Owner 也不能再手工增删成员（服务账号除外）。管理员可以手工覆盖权限。用户同步每天一次，群组同步每小时一次。同步时 LDAP 不可达，会导致“All users are blocked”。
- **Keycloak**：LDAP 联邦有 READ_ONLY、WRITABLE、UNSYNCED 三种编辑模式。Group Mapper 会把 LDAP 某个分支下的群组映射为 Keycloak 群组，并同步成员关系。编辑模式要在创建 LDAP Provider 时就定好，之后改了也不会更新已经生成的 Mapper。
- 冲突的根源是同一个节点或同一条成员关系有两个写入方。**建议**：
  - 每个部门节点和每条成员关系都带上 `source = ldap | local`。`ldap` 来源的数据在本地只读，只由同步任务写入；`local` 来源的数据由管理员维护。这和 v1 里“LDAP 管理的字段只读”是同一个思路。
  - 同步不创建用户，不合并用户（ADR-0002/0004）。目录中还没有在本工具登录过的人，不会出现在成员列表里。
  - 同步时 LDAP 不可达，应该**中止并保留原状**，不能当作“全部删除”处理（GitLab 那种全员 blocked 是反面例子）。
  - 先确认公司的 LDAP 能不能表达部门：可能是 OU，可能是用户的 `department`/`ou` 属性，也可能是 groupOfNames。不确定的话，下一期先只做本地部门树，同步留到再下一期，并预留 `source` 和 `external_id` 字段。

## 2. 权限模型

### 2.1 参照：NIST RBAC（ANSI/INCITS 359-2004，2012 年修订）

标准由 Core RBAC（用户-角色指派、角色-权限指派、会话）、层级 RBAC、静态职责分离（SSD）、动态职责分离（DSD）几个组件组成，并定义了管理函数和审查函数（例如“某个角色指派给了谁”）。对本工具有用的是 Core RBAC 和审查函数。层级 RBAC 和职责分离（SoD）下一期都用不上。

### 2.2 固定角色还是自定义角色

- **Gitea**：角色固定（Owner 团队和 Admin 团队），普通团队可以按“单元”（Code/Issues/PR/Releases…）选 Read 或 Write。这相当于在固定的权限点上做组合。
- **GitLab**：访问级别固定（Guest…Owner）。自定义成员角色要基于一个基础访问级别（LDAP group link 可以指定“default role or custom member role”）。
- **Keycloak / Casdoor**：角色和权限完全可配。这是 IdP 的定位：它不知道下游业务有哪些动作。
- 本工具是业务系统，动作是有限的，而且由代码定义（编辑应用、管理维护线、登记版本、改部署信息、改 Owner/PE…）。**建议**：
  - **权限点在代码里定义**，例如 `app.edit`、`line.fork`、`version.create`、`app.deploy_info.edit`、`app.members.edit`、`admin.platform.manage`、`admin.user.manage`、`admin.org.manage`。
  - **角色 = 一组权限点**。内置角色（管理员、Owner、PE，可再加“只读审计员”）作为不可删除的种子数据。
  - 自定义角色（管理员勾选权限点组合出一个新角色）**作为第二步**。表结构从第一天起就支持，但界面可以晚一些再做。先盘点有多少真实需求超出了 Owner/PE。

### 2.3 全局角色与资源范围角色

角色要标明自己的**作用范围**：`scope_type = global | application`。管理员只能在 global 授予，Owner/PE 只能在 application 授予。授权记录的形式如下：

```
role_binding(id, subject_type user|department, subject_id,
             role_id, scope_type global|application, scope_id NULL|app_id,
             source manual|ldap, created_by, created_at)
```

这和 Casbin 的“RBAC with domains”是同构的：`g = user, role, domain`，其中 domain 就是应用或 `*`。也和 Keycloak 的 realm role / client role（“a role namespace dedicated to the client”）、Zitadel 的“organization → project → roles/grants”同构。

### 2.4 授权给部门

- Keycloak 的群组可以挂角色映射，子群组会继承父群组的角色映射。authentik 的文档描述的是把角色授给群组。
- **建议**：`subject_type=department` 表示把角色授予这个部门**及其所有下级部门**的成员。计算有效权限时，用户的部门集合要向上展开到所有祖先部门。
- **和 ADR-0003 的冲突必须先定下来**。“至少一个 Owner”“不能禁用唯一 Owner”这两条，如果部门授权也能产生 Owner，就会变得含糊：部门换了人，Owner 就悄悄变了。**建议**：Owner 只允许直接授给用户，部门授权只能授 PE 或只读类角色；或者允许授，但 Owner 不变式只统计直接授给用户的 Owner。前者更简单，倾向于前者。这需要单独写一条 ADR。

### 2.5 数据范围（例如“只看我部门的应用”）

- 这类需求本质上是**列表过滤**，不是“能不能做”的判断。Casbin 的 `Enforce(sub, dom, obj, act)` 只判断单个对象。它确实也能枚举（`GetAllDomains`、`GetRolesForUserInDomain`、`GetImplicitRolesForUser` 等），可以拿到用户有角色的应用 ID 集合，但应用列表的分页、搜索、排序仍然要在 SQL 里用这个 ID 集合去过滤。部门树的继承也要先把部门成员关系镜像成 `g` 策略，这等于多了一份需要和业务表保持一致的数据。
- 所以无论用不用 Casbin，“我能看到哪些应用”最终都落在 SQL 上：递归 CTE 展开用户的部门，JOIN `role_binding`，再 JOIN 应用。
- 如果要按部门划分数据范围，**应用需要有一个归属部门**（`application.dept_id`）。这是一个新字段，也是一个产品决定：应用归部门，还是只归人？
- **建议**：v-next 先不做“数据范围”这个通用概念。可见性规则写死：管理员看全部；其他人看自己有授权的应用 + 全部应用的只读列表（v1 是否所有人可见，需要确认）。等应用有了归属部门，再加“部门负责人可见本部门及下级部门的应用”这一条固定规则。

### 2.6 用 Casbin 还是自己建表

| | Casbin（Go） | 自建表 + 代码里的权限点 |
|---|---|---|
| 单次判定 | `Enforce`，模型可配 | 一条 SQL 或一个内存 map |
| 列表过滤 / 数据范围 | 能枚举应用 ID 集合，但分页、搜索仍然要在 SQL 里做 | 天然就是 SQL |
| 部门继承 | 要把部门展开成 `g` 关系，或者自己写 RoleManager | 递归 CTE |
| 审计、外键、事务 | 策略存在 `casbin_rule` 宽表里，没有外键，和业务表的一致性要自己保证 | 和业务数据在同一个事务里 |
| 适用场景 | 规则经常变、多租户、ABAC 条件 | 动作固定、规模小 |

- **维护状况**（2026-10 核实）：仓库已迁到 `apache/casbin`。Go 模块路径从 v3.0.0（2025-12-09）起改为 `github.com/casbin/casbin/v3`，v2 停在 v2.135.0（2025-12）。最新版是 v3.11.0（2026-08-20），仍在活跃开发。`casbin/gorm-adapter` v3.41.0 已经依赖 casbin v3。**如果要用，新代码直接用 v3，不要用 v2。**
- 其他 Go 选项：OpenFGA（v1.22.0，2026-10，Zanzibar 关系模型，需要单独部署服务）、Ory Keto（Zanzibar，独立服务）、OPA（v1.21.2，Rego 策略引擎）、mikespook/gorbac（轻量的内存 RBAC）。对于一个单体内部工具，前三个都太重了。
- **建议**：**自己建表**（`role`、`role_permission`、`role_binding`、`department`、`user_department`），权限点是代码里的常量。只有在“自定义角色 + 条件规则”成为硬需求时，才考虑引入 Casbin v3 做判定层，而且即使那时，列表过滤也仍然要自己写 SQL。

## 3. 同类产品怎么划分“身份 / 归属 / 权限”

| 产品 | 身份（你是谁） | 归属（你在哪） | 权限（你能做什么） |
|---|---|---|---|
| Keycloak | User + 联邦身份（LDAP 等） | Group 树（子群组继承父群组的角色映射）；另有 Organizations 功能（目录中列出按域名和邀请管理成员） | Realm role / client role / composite role |
| Zitadel | 组织内的 User | Organization，扁平、不嵌套，类似租户 | 项目下的 roles，通过 grant 授权，可以跨组织委托 |
| Casdoor | User | Organization → Group 树（Physical/Virtual） | Role + 基于 Casbin 模型的 Permission |
| authentik | User + Source（LDAP 等） | Group（子群组成员算作父群组成员） | Role（授给 Group）+ 全局权限 / 对象权限 |
| GitLab | User + Identity（LDAP） | Group/Subgroup，可以和 LDAP 群组关联 | 固定访问级别 + 基于基础级别的自定义角色 |
| Gitea | User | Organization → Team | Team 对“单元”的 Read/Write/Admin |

共同点是三层彼此独立：**身份层**只回答“这是谁”，v1 的用户和登录身份已经是这样；**归属层**是树或群组，可以由目录同步；**权限层**是“主体 × 角色 × 作用范围”的授权记录，主体可以是人，也可以是群组或部门。GitLab 和 Gitea 这类业务系统以固定角色加资源范围为主，IdP 才做全可配置。

### 建议的 v-next 最小范围

1. 本地维护的部门树（邻接表），用户有一个主部门和若干附加部门，部门可以有负责人；v1 的部门字符串按上面的办法迁移。
2. 权限点常量；角色表，三个内置角色作为种子数据；`role_binding` 支持“用户或部门 × 全局或应用”。
3. 允许把 PE 或只读类角色授给部门（含下级部门）；Owner 只授给个人（需要一条新 ADR）。
4. 管理后台增加“部门”和“角色”两页，角色页先只读，用来展示每个角色包含哪些权限点；“某用户的有效权限及其来源”页面对应 NIST 的审查函数。
5. 审计（见下一节）。
6. **不做**：自定义角色编辑器、通用数据范围、LDAP 部门同步、角色层级、职责分离（SoD）、Casbin。表结构为这些预留字段（`source`、`external_id`、`role.builtin`）。

## 4. 权限变更审计

- 沿用并扩展 v1 的“变更历史”，不另起一套日志。应用范围的授权写进该应用的变更历史；全局授权和部门、角色定义的变更，写进管理后台的“系统审计”视图。两者可以共用一张 append-only 的表：

```
audit_event(id, at, actor_user_id NULL, actor_kind user|system_sync,
            action grant|revoke|role.update|dept.move|member.add|...,
            subject_type, subject_id, role_id, scope_type, scope_id,
            before_json, after_json, source manual|ldap, request_id)
```

- 记录的是授权这个动作，不展开到每个用户：给部门授一次角色就是一条记录。要回答“某人在 T 时刻为什么有这个权限”，需要部门成员变更也有记录，这样才能回放出当时的状态。
- LDAP 同步造成的变更用 `actor_kind=system_sync` 记录，一批同步共用一个 `request_id`，方便和手工操作区分。
- 只能追加：不提供删除或修改审计记录的接口，应用账号对这张表只授予 INSERT/SELECT 权限。

## 待决问题（建议用 grill 或 ADR 定下来）

1. 公司 LDAP 里有没有可靠的部门结构？是 OU、属性，还是群组？
2. 应用要不要有归属部门？
3. Owner 能不能经由部门授予？
4. 普通用户能看到所有应用，还是只能看到有授权的应用？
5. 有没有具体场景需要超出管理员、Owner、PE 的自定义角色？

## Sources

- NIST RBAC 项目页（INCITS 359-2004 / 359-2012）：https://csrc.nist.gov/projects/role-based-access-control
- Ferraiolo, Kuhn, Sandhu，《RBAC Standard Rationale》（2007）：https://csrc.nist.gov/CSRC/media/Projects/Role-Based-Access-Control/documents/ferraiolo-kuhn-sandhu-07.pdf
- 《The NIST Model for Role-Based Access Control》（2000）：https://csrc.nist.gov/publications/detail/conference-paper/2000/07/26/the-nist-model-for-role-based-access-control-towards-a-unified-
- INCITS 359-2012：https://standards.incits.org/apps/group_public/project/details.php?project_id=1658
- Casbin RBAC with Domains：https://casbin.apache.org/docs/rbac-with-domains/
- Casbin v3.0.0 发布说明（模块路径变更）：https://github.com/casbin/casbin/releases/tag/v3.0.0
- Casbin v3.11.0：https://github.com/apache/casbin/releases/tag/v3.11.0
- casbin/gorm-adapter：https://github.com/casbin/gorm-adapter
- Keycloak 服务器管理指南（Groups、Roles、LDAP 编辑模式与 Mapper、Organizations）：https://www.keycloak.org/docs/latest/server_admin/index.html
- Zitadel Organizations：https://zitadel.com/docs/concepts/structure/organizations
- Casdoor Organization：https://casdoor.ai/docs/organization/overview/
- Casdoor 组织树（Physical/Virtual 群组）：https://casdoor.ai/docs/organization/organization-tree/
- Casdoor Permissions：https://casdoor.ai/docs/permission/overview/
- authentik Groups：https://docs.goauthentik.io/users-sources/groups/
- authentik Access control：https://docs.goauthentik.io/users-sources/access-control/
- GitLab LDAP synchronization：https://docs.gitlab.com/administration/auth/ldap/ldap_synchronization/
- GitLab 群组成员与 LDAP group links：https://docs.gitlab.com/user/group/access_and_permissions/
- Gitea Permissions：https://docs.gitea.com/usage/permissions
- MySQL 8.0 WITH（递归 CTE）：https://dev.mysql.com/doc/refman/8.0/en/with.html
- OpenFGA：https://github.com/openfga/openfga
- Ory Keto：https://github.com/ory/keto
- OPA：https://github.com/open-policy-agent/opa
- gorbac：https://github.com/mikespook/gorbac
