import type { Connect, Component, Group, Overview, Tool } from './types'

const request = async <T,>(path: string, signal?: AbortSignal): Promise<T> => {
  const res = await fetch(path, { signal })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}
export const getOverview = (signal?: AbortSignal) => request<Overview>('/api/admin/overview', signal)
export const getConnects = async (signal?: AbortSignal) => (await request<{ connects: Connect[] }>('/api/admin/connects', signal)).connects
export const getComponents = async (signal?: AbortSignal) => (await request<{ components: Component[] }>('/api/admin/components', signal)).components
export const getComponent = (id: string, signal?: AbortSignal) => request<Component>(`/api/admin/components/${encodeURIComponent(id)}`, signal)
export const getGroups = async (signal?: AbortSignal) => (await request<{ groups: Group[] }>('/api/admin/groups', signal)).groups
export const getGroupTools = async (id: string, signal?: AbortSignal) => (await request<{ tools: Tool[] }>(`/api/admin/groups/${encodeURIComponent(id)}/tools`, signal)).tools
export const createGroup = async (group: Group) => { const res=await fetch('/api/admin/groups',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(group)}); if(!res.ok) throw new Error(await res.text()); return res.json() as Promise<Group> }
export const deleteGroup = async (id: string) => { const res=await fetch(`/api/admin/groups/${encodeURIComponent(id)}`,{method:'DELETE'}); if(!res.ok) throw new Error(await res.text()) }
export const attachTool = async (group: string, component: string, tool: string) => { const res=await fetch(`/api/admin/groups/${encodeURIComponent(group)}/tools`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({component_id:component,tool_name:tool})}); if(!res.ok) throw new Error(await res.text()) }
export const detachTool = async (group: string, component: string, tool: string) => { const res=await fetch(`/api/admin/groups/${encodeURIComponent(group)}/tools/${encodeURIComponent(component)}/${encodeURIComponent(tool)}`,{method:'DELETE'}); if(!res.ok) throw new Error(await res.text()) }
export const getTokenGroups = (tokenId:string, signal?:AbortSignal) => request<{tenant_id:string;default_group_id:string;group_ids:string[]}>(`/api/admin/tokens/${encodeURIComponent(tokenId)}/groups`,signal)
export const setTokenGroups = async (tokenId:string, defaultGroupId:string, groupIds:string[]) => { const res=await fetch(`/api/admin/tokens/${encodeURIComponent(tokenId)}/groups`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({default_group_id:defaultGroupId,group_ids:groupIds})}); if(!res.ok) throw new Error(await res.text()) }
