# 布局原型（PROTOTYPE，用完即弃）

回答的问题：第一期各页面应该长什么样，尤其是分支谱系视图怎么呈现。依据 `docs/design/v1-scope.md`「原型」一节，术语以 `GLOSSARY.md` 为准。

只存在于 `prototype/layout` 分支，不合入 main。选定方案后在正式代码里重写。

## 运行

```
pnpm prototype          # 仓库根目录；或 cd web-prototype && pnpm dev
```

打开 http://localhost:5199 。数据全部在内存里，刷新即重置。

- 当前状态：第一轮的四个方案（工作台 / 生命周期 / 版本流水 / 现状）已被否，代码在 git 历史 `a1a5f10`。`src/main.tsx` 现在只是占位页；新方案见 `RESEARCH.md`，选定后再做。
- 底部黑条（方案切换 + 视角切换）在 `src/prototype.tsx`，新方案会继续用。
- 假数据已换成欧美场景（`payments-core`、`order-service`、`fraud-engine`、`settlement-batch`），界面文案为英文，用 GLOSSARY 的英文名。`payments-core` 覆盖：4 层嵌套的维护线、同一版本分叉出两条线、Frozen 和 Retired 的线、分叉后还没有版本的线、不规则版本号（`emea-1.1`、`v2026.08`）、tag 和 commit 混用。`order-service` 的主线分支名是 `develop`；`settlement-batch` 的唯一 Owner 是只有本地身份的用户。
- 计划终止日期是可选字段，只出现在分叉对话框（可选输入）和维护线详情（一行普通键值）。没有倒计时、提醒、排序、变色，首页也没有相关内容。
- 假的仓库校验：路径里含 `missing` 会返回 404。

## 待确认

- 能否从已终止维护线上的版本分叉？范围文档说"任何线上的任何版本都可以作为源版本"，原型按"可以"处理。
- 状态能否回退（冻结 → 活跃）？原型只允许向前：活跃 → 冻结 → 终止。
- 应用唯一的 Owner 离职或长期不在时，谁来移交？范围文档只允许 Owner 增删 Owner/PE，管理员没有这个权限，禁用用户时会卡住。
- "谱系"不在 GLOSSARY 里，界面用了"分支谱系"作为区块名，可以交给 /domain-modeling 决定是否收录。
- 维护线创建后能否改名称和备注？范围文档没写，原型没做。
- 本地用户能否改自己的显示名 / 邮箱 / 部门？范围只说 LDAP 用户以 LDAP 为准。
- 没有记录分叉日期，只能用源版本的发布时间近似；终止也没有记录终止日期。
- `v1-scope.md` 里"即将到期的维护线"和"站内提醒"两处，和"弱化计划终止日期"冲突，见 `RESEARCH.md`。
- 主线没有状态（活跃 / 冻结 / 终止只用于维护线），请确认。
- GLOSSARY 没有"主线或维护线"的统称，方案里用了"线"。
- `@base-ui-components/react` 已改名为 `@base-ui/react`，原型用的是新包名，`v1-scope.md` 里的技术栈需要同步。
- 范围文档里的 "slug" 在 GLOSSARY 中叫"应用标识"，界面上统一用"应用标识"。
