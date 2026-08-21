import React, { useState } from 'react'
import { Link } from 'react-router-dom'

const quickStart = `mcp-connect login \\
  --gateway wss://mcp.example.com/tunnel

mcp-connect add docs \\
  --transport streamable-http \\
  --url http://127.0.0.1:8765/mcp`

export function LandingPage() {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    await navigator.clipboard.writeText(quickStart)
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1600)
  }
  return <div className="landing-page" id="top">
    <header className="landing-nav">
      <a className="landing-brand" href="#top" aria-label="MCPHub 首页"><img src="/brand/mcphub-mark.png" alt=""/><b>MCPHub</b></a>
      <nav aria-label="落地页导航"><a href="#capabilities">产品能力</a><a href="#discovery">动态发现</a><a href="#quick-start">快速开始</a></nav>
      <Link className="button button-ghost" to="/admin">进入管理中心</Link>
    </header>

    <main>
      <section className="hub-hero">
        <div className="hub-hero-copy">
          <div className="hero-badge"><span/> LOCAL CONNECTOR · CLOUD GATEWAY</div>
          <p className="hero-kicker">Connect local. Govern globally.</p>
          <h1>连接本地 MCP，<br/><em>让工具安全抵达。</em></h1>
          <p className="hero-lead">通过持久 Tunnel 将本地与内网 MCP 连接到统一 Gateway，集中完成路由、授权、工具分组和动态发现，让 Claude、Codex 与任意 Agent 按需使用。</p>
          <div className="hero-actions"><Link className="button button-primary" to="/admin">进入管理中心 <span>→</span></Link><a className="button button-secondary" href="#quick-start">查看快速开始</a></div>
          <div className="hero-proof"><span>✓ 无需开放本机端口</span><span>✓ stdio 与 HTTP</span><span>✓ 按需发现工具</span></div>
        </div>
        <div className="tunnel-visual" aria-label="MCPHub 连接拓扑预览">
          <div className="visual-grid"/><div className="tunnel-line line-left"/><div className="tunnel-line line-right"/>
          <div className="node node-local"><i>⌘</i><span>Local MCP<small>stdio · private</small></span></div>
          <div className="node node-server"><i>◇</i><span>Internal MCP<small>streamable HTTP</small></span></div>
          <div className="gateway-card"><img src="/brand/mcphub-mark.png" alt=""/><b>MCP Gateway</b><small>Connected · 12 tools</small><div><span>Auth</span><span>Groups</span><span>Routing</span></div></div>
          <div className="client-stack"><span>Claude</span><span>Codex</span><span>Agent</span></div>
          <div className="tunnel-packet packet-one">tools/list</div><div className="tunnel-packet packet-two">invoke ✓</div>
        </div>
      </section>

      <section className="flow-strip" aria-label="MCPHub 数据流"><div><span>01</span><b>Local MCP</b><small>工具留在原环境</small></div><i>→</i><div><span>02</span><b>mcp-connect</b><small>持久安全 Tunnel</small></div><i>→</i><div><span>03</span><b>MCPHub Gateway</b><small>路由、授权与治理</small></div><i>→</i><div><span>04</span><b>AI Clients</b><small>随处调用工具</small></div></section>

      <section className="landing-section" id="capabilities">
        <div className="section-heading"><p>WHY MCPHUB</p><h2>从散落的 MCP，<br/>到可连接、可授权的工具网络。</h2><span>不暴露本机端口，不把所有工具塞进上下文，也不让客户端绕过治理边界。</span></div>
        <div className="feature-grid">
          <article><div className="feature-icon">⌁</div><h3>本地安全连接</h3><p>mcp-connect 主动建立 WebSocket Tunnel，本地和内网服务无需开放入站端口。</p></article>
          <article><div className="feature-icon">⇄</div><h3>统一协议路由</h3><p>同时接入 stdio 和 Streamable HTTP，一条连接承载多个 MCP Component。</p></article>
          <article className="featured"><div className="feature-icon">⌕</div><h3>聚合动态发现</h3><p>Hub 初始只暴露 4 个元工具，Agent 搜索并按需加载真实 Tool Schema，显著节省上下文。</p></article>
          <article><div className="feature-icon">◎</div><h3>分组与 Token 授权</h3><p>按租户、Group 和 Token 限定工具发现与调用范围，并集中查看连接状态。</p></article>
        </div>
      </section>

      <section className="discovery-section" id="discovery">
        <div className="discovery-copy"><p>DYNAMIC DISCOVERY</p><h2>工具再多，<br/>上下文依然轻量。</h2><span>普通聚合器在初始化时向模型注入全部 Tool Schema。MCPHub 将工具目录留在 Gateway，只让 Agent 在需要时逐步发现。</span><div className="context-meter"><div><b>传统聚合</b><i><span style={{ width: '92%' }}/></i><small>全部工具进入上下文</small></div><div><b>MCPHub</b><i><span style={{ width: '22%' }}/></i><small>4 个稳定元工具</small></div></div></div>
        <div className="discovery-console"><header><span><i/><i/><i/></span><b>Agent tool flow</b></header><ol><li><em>01</em><code>mcphub_search</code><span>搜索当前 Group 中的相关工具</span></li><li><em>02</em><code>mcphub_get</code><span>只读取所需工具的 Schema</span></li><li><em>03</em><code>mcphub_invoke</code><span>通过 Gateway 执行并审计调用</span></li><li><em>04</em><code>mcphub_set_group</code><span>在已授权 Group 间切换</span></li></ol><footer>✓ Tool context loaded on demand</footer></div>
      </section>

      <section className="landing-section quick-section" id="quick-start">
        <div className="section-heading"><p>GET CONNECTED</p><h2>三步，把本地工具接入 Gateway。</h2></div>
        <div className="quick-grid"><ol><li><span>01</span><div><h3>登录 Gateway</h3><p>通过 Device Code 或连接 Token 建立身份。</p></div></li><li><span>02</span><div><h3>添加 MCP 服务</h3><p>配置 stdio 命令或已有 Streamable HTTP 地址。</p></div></li><li><span>03</span><div><h3>授权并连接 Agent</h3><p>绑定工具分组，复制 Component 或 Hub 地址。</p></div></li></ol><div className="code-card"><header><span><i/><i/><i/></span><b>terminal</b><button onClick={() => void copy()}>{copied ? '已复制 ✓' : '复制'}</button></header><pre>{quickStart}</pre><footer><span>●</span> Tunnel connected · heartbeat active</footer></div></div>
      </section>

      <section className="landing-cta"><p>YOUR TOOLS, ONE SECURE GATEWAY</p><h2>连接每一个 MCP，<br/>只加载此刻需要的工具。</h2><div><Link className="button button-light" to="/admin">进入管理中心 →</Link><a className="button button-outline-light" href="mailto:support@r9s.ai">联系我们</a></div></section>
    </main>
    <footer className="landing-footer"><a className="landing-brand" href="#top"><img src="/brand/mcphub-mark.png" alt=""/><b>MCPHub</b></a><p>Connect local. Govern globally.</p><a href="mailto:support@r9s.ai">support@r9s.ai</a></footer>
  </div>
}
