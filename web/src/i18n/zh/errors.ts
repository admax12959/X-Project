// 每个后端错误码一个 key："error.<code>"（internal/domain/codes.go）。
export default {
  'error.required': '必填。',
  'error.invalid': '取值无效。',
  'error.badRequest': '请求格式不正确。',
  'error.unauthenticated': '登录已失效，请重新登录。',
  'error.forbidden': '你没有执行此操作的权限。',
  'error.notFound': '不存在。',
  'error.internal': '服务器出错，请稍后再试。',
  'error.network': '无法连接服务器。',
  'error.unknown': '未知错误（{code}）。',
  'error.invalidCredentials': '用户名或密码不正确。',
  'error.usernameTakenByLocal': '该用户名已被本地用户占用，请联系管理员。',
  'error.usernameExistsInLdap': '该用户名在 LDAP 中已存在。',
  'error.ldapUnreachable': '无法连接 LDAP 目录，请稍后再试。',
};
