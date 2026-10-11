// 变更历史事件：每个后端 EventType（internal/domain/history.go）对应一个 "history.event.<type>"。
// 占位符由 web/src/history/render.ts 填充，用户输入的内容原样显示。
export default {
  'history.event.appCreated': '创建应用，Owner：{owners}',
  'history.event.appInfoChanged': '修改应用基本信息：{fields}',
  'history.event.deployInfoChanged': '修改部署信息',
  'history.event.memberAdded': '添加 {role}：{user}',
  'history.event.memberRemoved': '移除 {role}：{user}',
  'history.event.versionRegistered': '在{line}登记版本 {number}',
  'history.event.unknown': '未知事件（{type}）',
  'history.col.time': '时间',
  'history.col.by': '操作人',
  'history.col.event': '事件',
  'history.empty': '还没有变更记录。',
  'history.loadFailed': '变更历史加载失败。',
};
