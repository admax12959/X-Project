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
};
