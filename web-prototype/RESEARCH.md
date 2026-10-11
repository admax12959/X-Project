# 设计调研（PROTOTYPE 分支文档）

起因：第一轮三套方案被否——整体风格像 AI 生成、过度强调"到期"、演示数据偏亚太。本轮先看真实产品，再重做。

截图在 `/tmp/proto-refs/`（第三方页面，不入库）。完整逐项记录：`/tmp/proto-refs/cloud-consoles.md`、`/tmp/proto-refs/dev-tools.md`。

## 看了什么

| 参考 | 看到的 | 可信度 |
| --- | --- | --- |
| AWS Cloudscape | 列表页、详情页（标题 + 键值容器 + Tab）、可展开表格、拆分面板 | 高（实测计算样式；注意新版圆角偏大，不学） |
| Azure / Fluent 2 | 资源页（命令栏 + Essentials 键值面板）、列表页、DataGrid、Tree | 中（门户要登录，用的是官方文档截图） |
| Oracle Redwood | 文件夹树 + 表格、工具栏、暖灰配色 | 中低（官网内容没加载出来，用 Redwood 产品文档截图；没看到状态和详情页） |
| Backstage | 软件目录：列表 + 过滤栏、实体详情（About 键值卡、Relations 图） | 高，领域最接近 |
| GitHub / GitLab | 分支页、Tag 页、Release 页、提交网络图 | 高 |
| Azure DevOps、gitgraph.js、VS Code Git Graph | 分支图的画法 | 中（AzDO 只有 2017 年博客图） |
| Jira Releases、Linear | 只读了文档文字，没有产品截图 | 低，不作依据 |

## 共同点（几乎所有参考都一致）

- 白色或近白底，1px 细线分隔，容器没有阴影和渐变。
- 一个强调色（蓝），只用于链接、主按钮、当前 Tab；其余全是中性灰。颜色只留给状态。
- 正文 14px，标题只有 20–24px；层次靠字重和留白，不靠字号跳跃。
- 列表页固定结构：标题（带数量）→ 右侧操作 → 过滤行 → 表格（勾选 + 链接名称 + 属性 + 行操作）。行高 40–45px。
- 详情页：名称 + 操作在标题行，下面是键值块，再下面是 Tab。没有卡片套卡片。
  - Cloudscape：带边框容器，4 列"粗体标签在上、值在下"。
  - Azure：Essentials 面板，"标签：值"两列。
  - Backstage：标题下一行元数据条（Lifecycle / Owner / System）+ About 卡的 3 列键值。
- 状态：图标 + 文字（Cloudscape 4 种颜色 4 种图标形状），或纯文字（Azure、Backstage 的 Lifecycle）。没有实心胶囊。
- 层级：展开箭头 + 缩进，没有连接线；选中行用浅蓝底。

## 分支谱系视图：参考里没人直接做过

- GitHub Network：横向时间轴，没有版本标签，大片空白。
- GitLab Repository graph：每条线一种饱和色，颜色循环复用，**同一种颜色在不同位置代表不同分支**（彩虹问题），标签被截断。
- Azure DevOps：只有蓝和灰两种色，干净，但图里没有分支名。
- gitgraph.js：默认"主干灰、高亮分支一种颜色"，值得学；粗曲线 + 大圆点太玩具。
- 它们画的是提交，我们要画的是"版本 → 分叉出的维护线"，粒度不同。
- 结论：用位置和文字区分线，不用颜色区分；主线用中性深灰，版本刻度写成文字；只有被选中的线用强调色。标签放在图右侧同一行的文字列里，不浮在图上。

## 要避免的

- 状态用实心胶囊或一堆彩色徽章。
- 每个分支名都套浅蓝等宽字胶囊（GitHub 分支页，超过 20 个就吵）。
- 每列都带排序三角和拖拽手柄（Cloudscape），只在悬停或已排序列显示。
- Cloudscape 新版 16px 圆角和胶囊按钮、Azure 的蓝色大顶栏、Redwood 的装饰条和底部导航。
- Backstage 窄名称列导致名称断词、行高不齐，导航项 48px 过松。

## 两个方案（结构不同，不只是换颜色）

用词以 `GLOSSARY.md` 英文名为准：Application Key、Maintenance Line、Source Version、Fork Reason、Active / Frozen / Retired、Mainline、Login Identity。

### A. Console（参考 Cloudscape + Azure）

- 框架：左侧导航 + 细顶栏（中性，不用品牌色大条）。
- 列表：标题"Applications (4)"+ 右侧主按钮；Azure 式过滤行（可移除的过滤项）+ "Showing 1–4 of 4"；表格行高 40px。
- 详情：标题 + 命令栏（Fork、Register version、Edit 等图标加文字）→ Essentials 键值面板 → Tab：Overview / Lineage / Versions / Change history。
- 谱系：**可展开的树形表格**（Cloudscape expandable table）。每行一条线：Name · Status · Forked from（源版本）· Latest version · Versions。点选一行，右侧拆分面板显示该线的 Fork Reason、备注、版本列表。可选"Graph"开关，开启后左侧多一条窄的图列。
- 首页原"即将到期"位置：改为 **Recent activity**（最近登记的版本、最近的分叉），不带时间压力。
- 色板：冷中性（白、#f6f7f8 灰、#c6c6cd 细线），强调蓝 #006ce0 一类；字体系统无衬线 14px，版本号和分支名用等宽 12px。

### B. Catalog（参考 Backstage + GitLab / GitHub 分支页）

- 框架：深色窄侧栏 + 浅灰画布 + 白色面板（Backstage 式），但导航项间距收紧到 32px。
- 列表：左侧过滤栏（Owned by me / Status / Code platform）+ 右侧表格，名称列保证不断词。
- 详情：标题下一行元数据条（Owner · PE · Code platform · Mainline），Tab 在下方；Overview 用 About 卡的 3 列键值。
- 谱系：**主线脊柱 + 分叉肘线**。主线是一条中性深灰竖线，版本是脊柱上的文字刻度；每条维护线从对应刻度拐出，成为一行 GitLab 式双行条目：第一行线名 + 分支名，第二行 12px 元信息（Source Version · Fork Reason 摘要 · 最新版本 · 日期）。全灰，只有选中的线用强调色。
- 首页原"即将到期"位置：改为 **Needs attention**：Frozen 的线、还没有登记任何版本的线。这些数据本来就有，不是新指标。
- 色板：暖中性（参考 Redwood：#fbf9f8 底、#e2e0df 细线、#161513 文字），强调色用沉稳的青蓝。

### 状态显示（两个方案共用）

图标 + 文字，四种形状：Active 绿色对勾圆圈，Frozen 蓝色（有意为之，不是警告）暂停圆圈，Retired 灰色减号圆圈。没有实心胶囊。

## 不做的事

- 不再用 hallmark 的主题目录选字体和配色（上一轮的朱红 / Archivo / IBM Plex 就是这么来的）；这轮颜色和字号直接从上面的参考里取。
- 没有"到期"、"剩余天数"、"跑道"、倒计时、按日期排序或变色、首页提醒。计划终止日期保留为**可选字段**：分叉对话框里一个可选输入，维护线详情里一行普通的键值（为空显示 —）。

## 待用户确认

1. **界面语言**：按"市场在欧美"，本轮默认英文界面（用 GLOSSARY 的英文名）。需要中文或中英双语请说明，文案要整体改。
2. **`docs/design/v1-scope.md` 里有两处和"弱化计划终止日期"冲突**（文档是你的未跟踪文件，我没动）：
   - "原型"第 2 条里的"即将到期的维护线"
   - "其他"里"维护线计划终止日期临近时，在站内提醒……"
   维护线字段里的"计划终止日期"可以保留，作为可选字段。
3. 选 A、选 B，还是取两者的一部分（例如 A 的框架 + B 的谱系）。
