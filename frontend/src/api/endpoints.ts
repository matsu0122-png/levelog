import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from './client'
import type {
  CompleteResponse,
  HistoryDay,
  MissionTemplate,
  MissionTemplateInput,
  StatsResponse,
  TodayResponse,
  UncompleteResponse,
  User,
  XPTransaction,
} from './types'

export const authApi = {
  register: (email: string, password: string, timezone: string) =>
    apiPost<User>('/api/auth/register', { email, password, timezone }),
  login: (email: string, password: string) => apiPost<User>('/api/auth/login', { email, password }),
  logout: () => apiPost<{ ok: boolean }>('/api/auth/logout'),
  me: () => apiGet<User>('/api/me'),
}

export const missionApi = {
  list: () => apiGet<MissionTemplate[]>('/api/missions'),
  create: (input: MissionTemplateInput) => apiPost<MissionTemplate>('/api/missions', input),
  update: (id: string, input: MissionTemplateInput) => apiPut<MissionTemplate>(`/api/missions/${id}`, input),
  remove: (id: string) => apiDelete<{ ok: boolean }>(`/api/missions/${id}`),
  setActive: (id: string, active: boolean) => apiPatch<{ ok: boolean }>(`/api/missions/${id}/active`, { active }),
  today: () => apiGet<TodayResponse>('/api/missions/today'),
  history: (days = 7) => apiGet<HistoryDay[]>(`/api/missions/history?days=${days}`),
}

export const dailyMissionApi = {
  complete: (id: string) => apiPost<CompleteResponse>(`/api/daily-missions/${id}/complete`),
  uncomplete: (id: string) => apiPost<UncompleteResponse>(`/api/daily-missions/${id}/uncomplete`),
}

export const statsApi = {
  get: () => apiGet<StatsResponse>('/api/stats'),
  xpHistory: () => apiGet<XPTransaction[]>('/api/xp/history'),
}
