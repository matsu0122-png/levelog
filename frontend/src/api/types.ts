export type Difficulty = 'EASY' | 'NORMAL' | 'HARD' | 'EXTREME'

export const DIFFICULTIES: Difficulty[] = ['EASY', 'NORMAL', 'HARD', 'EXTREME']

export const DIFFICULTY_XP: Record<Difficulty, number> = {
  EASY: 10,
  NORMAL: 20,
  HARD: 40,
  EXTREME: 80,
}

export const DIFFICULTY_LABEL: Record<Difficulty, string> = {
  EASY: 'EASY',
  NORMAL: 'NORMAL',
  HARD: 'HARD',
  EXTREME: 'EXTREME',
}

export type Weekday = 'MONDAY' | 'TUESDAY' | 'WEDNESDAY' | 'THURSDAY' | 'FRIDAY' | 'SATURDAY' | 'SUNDAY'

export const WEEKDAYS: Weekday[] = ['MONDAY', 'TUESDAY', 'WEDNESDAY', 'THURSDAY', 'FRIDAY', 'SATURDAY', 'SUNDAY']

export const WEEKDAY_LABEL: Record<Weekday, string> = {
  MONDAY: '月',
  TUESDAY: '火',
  WEDNESDAY: '水',
  THURSDAY: '木',
  FRIDAY: '金',
  SATURDAY: '土',
  SUNDAY: '日',
}

export interface User {
  id: string
  email: string
  timezone: string
}

export interface Progress {
  level: number
  totalXp: number
  xpIntoLevel: number
  xpForNextLevel: number
}

export type DailyMissionStatus = 'PENDING' | 'COMPLETED'

export interface DailyMission {
  id: string
  missionTemplateId: string
  targetDate: string
  title: string
  xpReward: number
  status: DailyMissionStatus
  completedAt?: string
}

export interface TodayResponse {
  date: string
  missions: DailyMission[]
  completedCount: number
  totalCount: number
  progress: Progress
}

export interface HistoryDay {
  date: string
  missions: DailyMission[]
  completedCount: number
  totalCount: number
  xpEarned: number
}

export interface CompleteResponse {
  mission: DailyMission
  xpGained: number
  leveledUp: boolean
  progress: Progress
}

export interface UncompleteResponse {
  mission: DailyMission
  progress: Progress
}

export interface StatsResponse {
  progress: Progress
  todayCompleted: number
  todayTotal: number
}

export interface MissionTemplate {
  id: string
  title: string
  description: string
  difficulty: Difficulty
  xpReward: number
  active: boolean
  days: Weekday[]
  createdAt: string
  updatedAt: string
}

export interface MissionTemplateInput {
  title: string
  description: string
  difficulty: Difficulty
  days: Weekday[]
  active: boolean
}

export interface XPTransaction {
  id: string
  dailyMissionId: string
  amount: number
  transactionType: 'COMPLETE' | 'UNCOMPLETE'
  createdAt: string
}
