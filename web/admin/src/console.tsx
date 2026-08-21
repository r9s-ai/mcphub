import React, { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'
import { attachTool, createGroup, deleteGroup, detachTool, getComponent, getComponents, getConnects, getGroupTools, getGroups, getOverview, getTokenGroups, refreshCatalog, setTokenGroups } from './api'
import { useAuth } from './auth'
import type { Component, Connect, Group, Overview, Tool } from './types'

const ago = (value?: string) => {
  if (!value) return '暂无'
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000))
  if (seconds < 30) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  return new Date(value).toLocaleString()
}
const title = (value: string) => { useEffect(() => { document.title = `${value} · MCPHub` }, [value]) }
const Status = ({ value }: { value: string }) => <span className={`status ${value}`}><i/>{value === 'online' ? '在线' : '离线'}</span>
const Alert = ({ children, success = false }: { children: React.ReactNode; success?: boolean }) => <div className={`alert ${success ? 'success' : ''}`} role={success ? 'status' : 'alert'}>{children}</div>
const Loading = ({ text = '正在加载…' }: { text?: string }) => <div className="panel-state"><span className="spinner"/>{text}</div>
const Empty = ({ title: heading, detail }: { title: string; detail: string }) => <div className="empty-state"><span>◇</span><h3>{heading}</h3><p>{detail}</p></div>

export function LoginPage() {
  title('登录管理中心')
  const auth = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const redirect = new URLSearchParams(location.search).get('redirect') || '/admin'
  const safeRedirect = redirect.startsWith('/admin') && !redirect.startsWith('//') ? redirect : '/admin'
  useEffect(() => { if (!auth.loading && auth.signedIn) navigate(safeRedirect, { replace: true }) }, [auth.loading, auth.signedIn, navigate, safeRedirect])
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setSubmitting(true); setError('')
    try { await auth.login(token); navigate(safeRedirect, { replace: true }) }
    catch (value) { setError(value instanceof Error ? value.message : '登录失败') }
    finally { setSubmitting(false) }
  }
  if (auth.loading || auth.signedIn) return <div className="full-page-state"><span className="spinner"/><p>正在验证管理权限…</p></div>
  return <div className="login-page"><div className="login-grid"/><Link className="login-back" to="/">← 返回 MCPHub 首页</Link><section className="login-panel"><div className="login-brand"><img src="/brand/mcphub-mark.png" alt=""/><b>MCPHub</b></div><p className="eyebrow">GATEWAY ADMIN</p><h1>管理你的 MCP 网络</h1><p>使用 Gateway 的 Admin Token 登录，查看连接、服务、工具分组和访问授权。</p>{error && <Alert>{error}</Alert>}<form onSubmit={event => void submit(event)}><label>Admin Token<input type="password" value={token} onChange={event => setToken(event.target.value)} required autoFocus autoComplete="current-password" placeholder="输入 MCP_ADMIN_TOKEN"/></label><button className="button button-primary" disabled={submitting}>{submitting ? '正在验证…' : '进入管理中心 →'}</button></form><small>管理凭据仅保存在当前浏览器标签页中</small></section></div>
}

const menu = [
  ['/admin', '⌂', '概览'], ['/admin/connects', '⌁', '连接实例'], ['/admin/components', '◇', 'MCP 服务'], ['/admin/groups', '⌘', '工具分组'], ['/admin/tokens', '◎', 'Token 授权'],
] as const

export function ConsoleLayout() {
  const auth = useAuth()
  const [open, setOpen] = useState(false)
  const location = useLocation()
  useEffect(() => setOpen(false), [location.pathname])
  return <div className="console-shell">
    <aside className={`console-sidebar ${open ? 'open' : ''}`}><div className="console-brand"><img src="/brand/mcphub-mark.png" alt=""/><div><b>MCPHub</b><small>Gateway Console</small></div><button className="mobile-close" aria-label="关闭菜单" onClick={() => setOpen(false)}>×</button></div><nav><p>GATEWAY</p>{menu.slice(0, 3).map(([to, icon, label]) => <NavLink key={to} to={to} end={to === '/admin'}><span>{icon}</span>{label}</NavLink>)}<p>ACCESS CONTROL</p>{menu.slice(3).map(([to, icon, label]) => <NavLink key={to} to={to}><span>{icon}</span>{label}</NavLink>)}</nav><div className="sidebar-bottom"><Link to="/">← 产品首页</Link>{auth.mode === 'token' ? <button onClick={() => auth.logout()}>退出登录</button> : <button disabled>开发模式无需登录</button>}</div></aside>
    {open && <button className="sidebar-scrim" aria-label="关闭菜单" onClick={() => setOpen(false)}/>}
    <div className="console-workspace"><header className="console-topbar"><button className="mobile-menu" aria-label="打开菜单" onClick={() => setOpen(true)}>☰</button><div><span className="live-dot"/> Gateway Control Plane</div><div><span className={`mode-badge ${auth.mode}`}>{auth.mode === 'development' ? '开发模式' : 'Admin Token'}</span><span className="tenant-badge">全局 Gateway</span></div></header><main className="console-main"><Outlet/></main></div>
  </div>
}

function PageHeading({ kicker, heading, description, action }: { kicker: string; heading: string; description: string; action?: React.ReactNode }) {
  return <div className="page-heading"><div><p>{kicker}</p><h1>{heading}</h1><span>{description}</span></div>{action}</div>
}

export function OverviewPage() {
  title('概览')
  const [overview, setOverview] = useState<Overview>()
  const [connects, setConnects] = useState<Connect[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const refresh = useCallback(async () => { setLoading(true); try { const [o, c] = await Promise.all([getOverview(), getConnects()]); setOverview(o); setConnects(c); setError('') } catch (e) { setError((e as Error).message) } finally { setLoading(false) } }, [])
  useEffect(() => { void refresh() }, [refresh])
  const components = connects.flatMap(item => item.components)
  return <><PageHeading kicker="GATEWAY OVERVIEW" heading="连接状态概览" description="统一观察 Connect 实例、MCP 服务和 Gateway 实时状态。" action={<button className="button button-muted" onClick={() => void refresh()} disabled={loading}>↻ 刷新状态</button>}/>{error && <Alert>状态 API 暂时不可用：{error}</Alert>}<section className="metric-grid">{[['连接实例', overview?.connect_total ?? '—', `${overview?.connect_online ?? 0} 个在线`], ['MCP 服务', overview?.component_total ?? '—', `${overview?.component_online ?? 0} 个在线`], ['在线率', overview?.component_total ? `${Math.round((overview.component_online / overview.component_total) * 100)}%` : '—', '按服务计算'], ['异常服务', overview?.component_offline ?? '—', '需要检查连接']].map(([label, value, hint]) => <article key={label}><span>{label}</span><b>{value}</b><small>{hint}</small></article>)}</section><section className="dashboard-grid"><div className="panel-card"><div className="card-heading"><div><h2>最近连接实例</h2><p>心跳与所承载的 Component。</p></div><Link to="/admin/connects">查看全部 →</Link></div>{loading && !connects.length ? <Loading/> : connects.length ? connects.slice(0, 5).map(item => <div className="compact-row" key={item.id}><Status value={item.status}/><span><b>{item.name}</b><small>{item.id} · {item.components.length} 个服务</small></span><time>{ago(item.last_heartbeat)}</time></div>) : <Empty title="暂无连接实例" detail="运行 mcp-connect login 后，实例会出现在这里。"/>}</div><div className="panel-card"><div className="card-heading"><div><h2>服务健康</h2><p>离线与最近异常优先显示。</p></div><Link to="/admin/components">服务列表 →</Link></div>{components.length ? components.sort((a, b) => Number(a.status === 'online') - Number(b.status === 'online')).slice(0, 5).map(item => <div className="compact-row" key={item.id}><Status value={item.status}/><span><b>{item.name}</b><small>{item.transport}{item.last_error ? ` · ${item.last_error}` : ''}</small></span><time>{ago(item.last_heartbeat)}</time></div>) : <Empty title="暂无 MCP 服务" detail="在 mcp-connect 中添加 stdio 或 HTTP Component。"/>}</div></section></>
}

export function ConnectsPage() {
  title('连接实例')
  const [items, setItems] = useState<Connect[] | null>(null); const [error, setError] = useState('')
  useEffect(() => { const controller = new AbortController(); getConnects(controller.signal).then(setItems).catch(e => { if (e.name !== 'AbortError') setError(e.message) }); return () => controller.abort() }, [])
  return <><PageHeading kicker="MCP CONNECT" heading="连接实例" description="查看通过持久 WebSocket Tunnel 接入 Gateway 的 mcp-connect 实例。"/>{error && <Alert>{error}</Alert>}<section className="connect-grid">{items === null ? <Loading/> : items.length ? items.map(item => <article className="panel-card connect-panel" key={item.id}><div><span className="connect-icon">⌁</span><div><h2>{item.name}</h2><code>{item.id}</code></div><Status value={item.status}/></div><dl><div><dt>版本</dt><dd>{item.version || '未知'}</dd></div><div><dt>远端地址</dt><dd>{item.remote_addr || '本地连接'}</dd></div><div><dt>最后心跳</dt><dd>{ago(item.last_heartbeat)}</dd></div><div><dt>MCP 服务</dt><dd>{item.components.length} 个</dd></div></dl><div className="component-chips">{item.components.map(component => <Link to={`/admin/components/${encodeURIComponent(component.id)}`} key={component.id}>{component.name}<Status value={component.status}/></Link>)}</div></article>) : <Empty title="尚无连接实例" detail="执行 mcp-connect login 连接此 Gateway。"/>}</section></>
}

function ComponentTable({ items }: { items: Component[] }) {
  return <div className="table-wrap"><table><thead><tr><th>服务</th><th>Transport</th><th>连接实例</th><th>状态</th><th>最后心跳</th><th>公开地址</th><th/></tr></thead><tbody>{items.map(item => <tr key={`${item.connect_id}:${item.id}`}><td><b>{item.name}</b>{item.last_error && <small className="error-text">{item.last_error}</small>}</td><td><span className="transport">{item.transport}</span></td><td><code>{item.connect_id}</code></td><td><Status value={item.status}/></td><td>{ago(item.last_heartbeat)}</td><td><code className="truncate-code">{item.public_url}</code></td><td><Link to={`/admin/components/${encodeURIComponent(item.id)}`}>详情 ›</Link></td></tr>)}</tbody></table></div>
}

export function ComponentsPage() {
  title('MCP 服务')
  const [items, setItems] = useState<Component[] | null>(null); const [error, setError] = useState('')
  useEffect(() => { const controller = new AbortController(); getComponents(controller.signal).then(setItems).catch(e => { if (e.name !== 'AbortError') setError(e.message) }); return () => controller.abort() }, [])
  return <><PageHeading kicker="MCP COMPONENTS" heading="MCP 服务" description="集中查看所有 stdio 与 Streamable HTTP 服务的路由和健康状态。"/>{error && <Alert>{error}</Alert>}<section className="panel-card table-card">{items === null ? <Loading/> : items.length ? <ComponentTable items={items}/> : <Empty title="尚无 MCP 服务" detail="在任一 mcp-connect 实例中添加 Component 后即可访问。"/>}</section></>
}

export function ComponentDetailPage() {
  const { id = '' } = useParams(); title('服务详情')
  const [item, setItem] = useState<Component>(); const [error, setError] = useState(''); const [notice, setNotice] = useState('')
  useEffect(() => { const controller = new AbortController(); getComponent(decodeURIComponent(id), controller.signal).then(setItem).catch(e => { if (e.name !== 'AbortError') setError(e.message) }); return () => controller.abort() }, [id])
  const copy = async () => { if (item) { await navigator.clipboard.writeText(item.public_url); setNotice('MCP 地址已复制') } }
  const refresh = async () => { if (!item) return; try { await refreshCatalog(item.id); setNotice('工具目录刷新已开始') } catch (e) { setError((e as Error).message) } }
  if (!item && !error) return <Loading text="正在加载服务详情…"/>
  return <><PageHeading kicker="COMPONENT DETAIL" heading={item?.name || '服务详情'} description="检查上游连接、公开路由与工具目录状态。" action={<Link className="button button-muted" to="/admin/components">← 返回列表</Link>}/>{error && <Alert>{error}</Alert>}{notice && <Alert success>{notice}</Alert>}{item && <section className="detail-grid"><div className="panel-card detail-card"><div className="detail-title"><div><span className="service-mark">◇</span><div><h2>{item.name}</h2><code>{item.id}</code></div></div><Status value={item.status}/></div><dl><dt>所属连接</dt><dd>{item.connect_id}</dd><dt>传输类型</dt><dd><span className="transport">{item.transport}</span></dd><dt>上游地址</dt><dd><code>{item.upstream_url || '未提供'}</code></dd><dt>注册时间</dt><dd>{new Date(item.registered_at).toLocaleString()}</dd><dt>最后心跳</dt><dd>{new Date(item.last_heartbeat).toLocaleString()}</dd>{item.last_error && <><dt>最近错误</dt><dd className="error-text">{item.last_error}</dd></>}</dl></div><aside className="panel-card route-card"><p>PUBLIC MCP ENDPOINT</p><h2>客户端连接地址</h2><code>{item.public_url}</code><button className="button button-primary" onClick={() => void copy()}>复制 MCP 地址</button><button className="button button-muted" onClick={() => void refresh()}>刷新工具目录</button><small>刷新会通过 Tunnel 重新请求 tools/list，并更新动态发现目录。</small></aside></section>}</>
}

export function GroupsPage() {
  title('工具分组')
  const [groups, setGroups] = useState<Group[]>([]); const [tools, setTools] = useState<Record<string, Tool[]>>({}); const [error, setError] = useState(''); const [name, setName] = useState(''); const [description, setDescription] = useState(''); const [binding, setBinding] = useState<Record<string, { component: string; tool: string }>>({}); const [loading, setLoading] = useState(true)
  const refresh = useCallback(async () => { setLoading(true); try { const list = await getGroups(); setGroups(list); const pairs = await Promise.all(list.map(async group => [group.id, await getGroupTools(group.id)] as const)); setTools(Object.fromEntries(pairs)); setError('') } catch (e) { setError((e as Error).message) } finally { setLoading(false) } }, [])
  useEffect(() => { void refresh() }, [refresh])
  const add = async () => { if (!name.trim()) return; try { await createGroup({ id: name.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '-'), name: name.trim(), description: description.trim(), is_default: groups.length === 0 }); setName(''); setDescription(''); await refresh() } catch (e) { setError((e as Error).message) } }
  return <><PageHeading kicker="DYNAMIC DISCOVERY" heading="工具分组" description="Hub 只向 Agent 暴露当前 Token 已授权 Group 中的工具，减少上下文并控制调用边界。"/>{error && <Alert>分组操作失败：{error}</Alert>}<section className="discovery-note"><div><span>⌕</span><div><b>按需发现，而不是一次加载全部工具</b><p>Agent 初始只看到 mcphub_search、mcphub_get、mcphub_invoke 和 mcphub_set_group。</p></div></div><code>search → get schema → invoke</code></section><section className="panel-card group-create"><div><h2>创建工具分组</h2><p>按团队、场景或权限边界组织 Tool Catalog。</p></div><input value={name} onChange={e => setName(e.target.value)} placeholder="分组名称，例如 coding"/><input value={description} onChange={e => setDescription(e.target.value)} placeholder="分组描述"/><button className="button button-primary" onClick={() => void add()}>创建分组</button></section>{loading && !groups.length ? <Loading/> : groups.length ? <div className="groups-grid">{groups.map(group => <article className="panel-card group-card" key={group.id}><header><div><h2>{group.name}{group.is_default && <span>默认</span>}</h2><p>{group.description || '暂无描述'} · <code>{group.id}</code></p></div><button className="danger-button" disabled={group.is_default} onClick={async () => { await deleteGroup(group.id); await refresh() }}>删除</button></header><div className="group-tools">{(tools[group.id] || []).map(tool => <span className="tool-chip" key={`${tool.component}:${tool.name}`}>{tool.component}.{tool.name}<button aria-label={`解绑 ${tool.name}`} onClick={async () => { await detachTool(group.id, tool.component, tool.name); await refresh() }}>×</button></span>)}{!(tools[group.id] || []).length && <span className="muted">该分组尚未绑定工具</span>}</div><div className="bind-row"><input placeholder="Component ID" value={binding[group.id]?.component || ''} onChange={e => setBinding(value => ({ ...value, [group.id]: { component: e.target.value, tool: value[group.id]?.tool || '' } }))}/><input placeholder="Tool name" value={binding[group.id]?.tool || ''} onChange={e => setBinding(value => ({ ...value, [group.id]: { component: value[group.id]?.component || '', tool: e.target.value } }))}/><button className="button button-muted" onClick={async () => { const item = binding[group.id]; if (!item?.component || !item.tool) return; await attachTool(group.id, item.component, item.tool); setBinding(value => ({ ...value, [group.id]: { component: '', tool: '' } })); await refresh() }}>绑定工具</button></div></article>)}</div> : <Empty title="暂无工具分组" detail="创建第一个 Group，开始使用聚合动态发现。"/>}</>
}

export function TokensPage() {
  title('Token 授权')
  const [groups, setGroups] = useState<Group[]>([]); const [tokenId, setTokenId] = useState(''); const [defaultGroup, setDefaultGroup] = useState(''); const [allowed, setAllowed] = useState<string[]>([]); const [message, setMessage] = useState(''); const [error, setError] = useState(''); const [loading, setLoading] = useState(false)
  useEffect(() => { getGroups().then(setGroups).catch(e => setError(e.message)) }, [])
  const load = async () => { if (!tokenId.trim()) return; setLoading(true); setError(''); try { const value = await getTokenGroups(tokenId.trim()); setDefaultGroup(value.default_group_id); setAllowed(value.group_ids); setMessage('已加载当前授权') } catch (e) { setError((e as Error).message) } finally { setLoading(false) } }
  const save = async () => { if (!tokenId.trim()) return; setLoading(true); setError(''); try { const selected = Array.from(new Set(defaultGroup ? [defaultGroup, ...allowed] : allowed)); await setTokenGroups(tokenId.trim(), defaultGroup, selected); setAllowed(selected); setMessage('Token 分组授权已保存') } catch (e) { setError((e as Error).message) } finally { setLoading(false) } }
  return <><PageHeading kicker="ACCESS TOKENS" heading="Token 授权" description="限制访问 Token 可发现、读取 Schema 和调用的工具分组。"/>{error && <Alert>{error}</Alert>}{message && <Alert success>{message}</Alert>}<section className="token-layout"><div className="panel-card token-form"><div className="card-heading"><div><h2>选择 Token</h2><p>使用数据库中保存的 Token ID（SHA-256 Hash），不输入明文 Access Token。</p></div></div><label>Token ID<div className="input-action"><input value={tokenId} onChange={e => setTokenId(e.target.value)} placeholder="token hash"/><button className="button button-muted" disabled={loading} onClick={() => void load()}>读取授权</button></div></label><label>默认工具分组<select value={defaultGroup} onChange={e => setDefaultGroup(e.target.value)}><option value="">请选择默认 Group</option>{groups.map(group => <option value={group.id} key={group.id}>{group.name}</option>)}</select></label><fieldset><legend>允许访问的分组</legend>{groups.map(group => <label className="check-row" key={group.id}><input type="checkbox" checked={allowed.includes(group.id)} onChange={e => setAllowed(value => e.target.checked ? [...value, group.id] : value.filter(id => id !== group.id))}/><span><b>{group.name}</b><small>{group.description || group.id}</small></span></label>)}</fieldset><button className="button button-primary" disabled={loading || !tokenId.trim()} onClick={() => void save()}>{loading ? '处理中…' : '保存授权'}</button></div><aside className="panel-card token-explain"><span>◎</span><h2>Group-scoped Discovery</h2><p>每个 Token 只能搜索和调用被授权 Group 中的工具。切换 Group 也必须在允许范围内完成。</p><ol><li>Token 确定默认 Group</li><li>mcphub_search 过滤目录</li><li>mcphub_invoke 再次校验权限</li></ol></aside></section></>
}
