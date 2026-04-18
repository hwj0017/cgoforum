import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { getApiBase } from '../api'
import { useForum } from '../app/useForum'

export default function ShellLayout() {
    const { hasToken, message, loading, handleLogout } = useForum()
    const navigate = useNavigate()

    async function onLogout() {
        await handleLogout()
        navigate('/auth')
    }

    return (
        <div className="shell">
            <header className="topbar">
                <div>
                    <p className="badge">CGOForum Web</p>
                    <h1>论坛工作台</h1>
                    <p className="sub">后端地址: {getApiBase()}</p>
                </div>
                <div className="auth-status">
                    <span>{hasToken ? '已登录' : '未登录'}</span>
                    {hasToken ? <button onClick={() => void onLogout()}>退出</button> : null}
                </div>
            </header>

            <div className="workspace">
                <aside className="nav-pane panel">
                    <h2>导航</h2>
                    <nav className="route-nav">
                        <NavLink to="/" end className={({ isActive }) => (isActive ? 'route-link active' : 'route-link')}>
                            发现页
                        </NavLink>
                        <NavLink to="/feed" className={({ isActive }) => (isActive ? 'route-link active' : 'route-link')}>
                            关注流
                        </NavLink>
                        <NavLink to="/compose" className={({ isActive }) => (isActive ? 'route-link active' : 'route-link')}>
                            创作中心
                        </NavLink>
                        <NavLink to="/auth" className={({ isActive }) => (isActive ? 'route-link active' : 'route-link')}>
                            账户中心
                        </NavLink>
                    </nav>
                </aside>

                <section className="content-pane">
                    {message ? <div className={`notice ${message.type}`}>{message.text}</div> : null}
                    {loading ? <div className="loading">处理中...</div> : null}
                    <Outlet />
                </section>
            </div>
        </div>
    )
}
