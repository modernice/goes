export interface Session {
  authenticated: boolean
  authenticationEnabled: boolean
  username?: string
}

export interface StoreInfo {
  id: string
  name: string
  driver: 'mongo' | 'postgres'
  status: 'online' | 'unavailable'
  error?: string
}

export interface AggregateRef {
  name: string
  id: string
  version: number
}

export interface StoredEvent {
  id: string
  name: string
  time: string
  aggregate?: AggregateRef
  data: unknown
  dataDecodeError?: string
}

export interface Summary {
  totalEvents: number
  eventTypes: number
  aggregateTypes: number
  aggregateStreams: number
  latestEventTime?: string
  payloadDecodeErrors?: number
}

export interface Facet {
  value: string
  count: number
}

export interface Facets {
  eventNames: Facet[]
  aggregateNames: Facet[]
}

export interface Stream {
  aggregateName: string
  aggregateId: string
  createdAt: string
}

export interface Page<T> {
  items: T[]
  nextCursor?: string
}

export interface APIErrorBody {
  error?: {
    code?: string
    message?: string
  }
}
