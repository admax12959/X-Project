// 管理后台 > 安全策略（#23）：是否要求全体用户启用二次验证。
// "security.field.<name>" 同时是审计日志里被修改字段的名称（API 字段名）。
export default {
  'security.title': '安全策略',
  'security.intro': '管理员账号始终必须启用二次验证，不能关闭。这里可以进一步要求全体用户启用。修改立即生效，包括已经登录的用户。',
  'security.loadFailed': '无法加载安全策略。',
  'security.field.forceSecondFactor': '全体用户强制二次验证',
  'security.force.label': '要求全体用户启用二次验证',
  'security.force.help': '开启后，还没有启用二次验证的用户登录后只能进入启用流程，完成之前其他功能都不可用；已启用的用户不能自行关闭。',
  'security.admins.note': '管理员始终强制：管理员账号必须启用二次验证，不受此开关影响。',
  'security.saved': '已保存。',
  'security.save': '保存',
  'security.saving': '保存中…',
};
