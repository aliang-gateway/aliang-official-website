/**
 * 扫码登录轮询的恢复决策。
 *
 * 2026-09-21 修复：扫码行在 expires_at + 10 分钟宽限后被服务端清理，之后
 * /api/auth/scan/status 对该 device_code 永远返回 404。原实现对 !res.ok 一律
 * 继续轮询同一个死码，登录页挂着一张永远无效的二维码（生产日志里观察到
 * 01:11 起连续 137 次 404 死轮询）。404 = 行已不存在，必须废弃当前会话重新
 * init 换新码；其余错误（5xx/429/瞬断）按原样继续轮询。
 */
export function scanPollNeedsNewSession(httpStatus: number): boolean {
  return httpStatus === 404;
}
