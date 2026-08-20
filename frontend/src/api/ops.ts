import { getJson, postJson } from './client'

export type OpsAccess = {
  allowed: boolean
  grafana_path: string
}

export type OpsLogLine = {
  time: string
  line: string
  labels?: string
}

export type OpsLogs = {
  query: string
  lines: OpsLogLine[]
}

export type OpsMetrics = {
  qps?: number | null
  error_rate?: number | null
}

export function opsAccess() {
  return getJson<OpsAccess>('/ops/access', { authRequired: true })
}

export function opsMetrics() {
  return getJson<OpsMetrics>('/ops/metrics', { authRequired: true })
}

export function opsLogs(query: string, sinceMinutes: number, limit = 100) {
  return postJson<OpsLogs>('/ops/logs', { query, since_minutes: sinceMinutes, limit }, { authRequired: true })
}
