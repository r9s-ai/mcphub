export type Status = 'online' | 'offline'
export type Component = { id: string; connect_id: string; name: string; transport: string; upstream_url?: string; status: Status; last_heartbeat: string; registered_at: string; public_url: string; last_error?: string }
export type Connect = { id: string; name: string; version?: string; status: Status; connected_at: string; last_heartbeat: string; remote_addr?: string; components: Component[] }
export type Overview = { connect_total: number; connect_online: number; component_total: number; component_online: number; component_offline: number; last_updated: string }
export type Tool = { component: string; name: string; description?: string; enabled: boolean }
export type Group = { id: string; name: string; description?: string; tags?: string[]; is_default: boolean }
