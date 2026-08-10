import { postJson } from './client'
import type { GetAllFollowersResponse, GetAllVloggersResponse } from './types'

export function follow(vloggerId: number) {
  return postJson<null>('/social/follow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function unfollow(vloggerId: number) {
  return postJson<null>('/social/unfollow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function getAllFollowers(vloggerId?: number) {
  return postJson<GetAllFollowersResponse>(
    '/social/getAllFollowers',
    vloggerId ? { vlogger_id: vloggerId } : {},
    { authRequired: true },
  )
}

export function getAllVloggers(followerId?: number) {
  return postJson<GetAllVloggersResponse>(
    '/social/getAllVloggers',
    followerId ? { follower_id: followerId } : {},
    { authRequired: true },
  )
}
