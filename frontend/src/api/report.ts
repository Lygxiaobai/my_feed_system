import { postJson } from './client'

/** 与后端 report.Reason 一致，改动时两边必须同步。 */
export type ReportReason =
  | 'spam'
  | 'porn'
  | 'violence'
  | 'illegal'
  | 'infringement'
  | 'misinformation'
  | 'abuse'
  | 'minor'
  | 'other'

/** 举报单自身的状态，与视频的 audit_status 不是一回事。 */
export type ReportStatus = 'pending' | 'accepted' | 'dismissed'

/**
 * 展示文案放前端、后端只校验枚举合法性。
 *
 * 顺序即展示顺序：把最常见的理由放前面，减少用户翻找。
 */
export const REPORT_REASONS: { value: ReportReason; label: string }[] = [
  { value: 'porn', label: '色情低俗' },
  { value: 'spam', label: '垃圾营销' },
  { value: 'abuse', label: '人身攻击' },
  { value: 'violence', label: '暴力血腥' },
  { value: 'misinformation', label: '虚假信息' },
  { value: 'infringement', label: '侵权抄袭' },
  { value: 'minor', label: '危害未成年人' },
  { value: 'illegal', label: '违法违规' },
  { value: 'other', label: '其他' },
]

/** 与后端 report.DetailMaxLength 一致。 */
export const REPORT_DETAIL_MAX = 500

export type MyReport = {
  id: number
  target_type: string
  target_id: number
  reason: ReportReason
  detail: string
  status: ReportStatus
  created_at: string
  handled_at: string | null
}

export async function reportVideo(input: { video_id: number; reason: ReportReason; detail?: string }) {
  const res = await postJson<{ report: { id: number; status: ReportStatus; created_at: string } }>(
    '/report/video',
    input,
    { authRequired: true },
  )
  return res.report
}

export async function listMyReports(params?: { limit?: number; offset_id?: number }) {
  const res = await postJson<{ items: MyReport[] }>('/report/mine', params ?? {}, { authRequired: true })
  return res.items ?? []
}

export type ReportAction = 'dismiss' | 'takedown'

export type PendingReportItem = {
  target_type: string
  target_id: number
  report_count: number
  reason_counts: Partial<Record<ReportReason, number>>
  firstly_at: string
  latest_at: string
  samples: string[]
}

export async function listPendingReports(limit = 20) {
  const res = await postJson<{ items: PendingReportItem[] }>('/report/pending', { limit }, { authRequired: true })
  return res.items ?? []
}

export async function handleReport(input: { video_id: number; action: ReportAction; note?: string }) {
  const res = await postJson<{ resolved: number }>('/report/handle', input, { authRequired: true })
  return res.resolved
}

export function reasonLabel(reason: ReportReason) {
  return REPORT_REASONS.find((r) => r.value === reason)?.label ?? reason
}
