import type { Connect, Component, Overview } from './types'

const request = async <T,>(path: string, signal?: AbortSignal): Promise<T> => {
  const res = await fetch(path, { signal })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}
export const getOverview = (signal?: AbortSignal) => request<Overview>('/api/admin/overview', signal)
export const getConnects = async (signal?: AbortSignal) => (await request<{ connects: Connect[] }>('/api/admin/connects', signal)).connects
export const getComponents = async (signal?: AbortSignal) => (await request<{ components: Component[] }>('/api/admin/components', signal)).components
export const getComponent = (id: string, signal?: AbortSignal) => request<Component>(`/api/admin/components/${encodeURIComponent(id)}`, signal)
