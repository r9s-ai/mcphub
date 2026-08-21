import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router-dom'
import { AuthProvider, ProtectedRoute } from './auth'
import { ComponentDetailPage, ComponentsPage, ConnectsPage, ConsoleLayout, GroupsPage, LoginPage, OverviewPage, TokensPage } from './console'
import { LandingPage } from './landing'
import './styles.css'

const AdminAuthBoundary = () => <AuthProvider><Outlet/></AuthProvider>

function App() {
  return <Routes>
    <Route path="/" element={<LandingPage/>}/>
    <Route element={<AdminAuthBoundary/>}>
      <Route path="/admin/login" element={<LoginPage/>}/>
      <Route element={<ProtectedRoute/>}>
        <Route element={<ConsoleLayout/>}>
          <Route path="/admin" element={<OverviewPage/>}/>
          <Route path="/admin/connects" element={<ConnectsPage/>}/>
          <Route path="/admin/components" element={<ComponentsPage/>}/>
          <Route path="/admin/components/:id" element={<ComponentDetailPage/>}/>
          <Route path="/admin/groups" element={<GroupsPage/>}/>
          <Route path="/admin/tokens" element={<TokensPage/>}/>
        </Route>
      </Route>
    </Route>
    <Route path="*" element={<Navigate to="/" replace/>}/>
  </Routes>
}

createRoot(document.getElementById('root')!).render(<React.StrictMode><BrowserRouter><App/></BrowserRouter></React.StrictMode>)
