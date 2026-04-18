import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForum } from '../app/useForum'

export default function AuthPage() {
    const { authForm, setAuthForm, handleLogin, handleRegister } = useForum()
    const navigate = useNavigate()

    async function onLogin(event: FormEvent<HTMLFormElement>) {
        const ok = await handleLogin(event)
        if (ok) {
            navigate('/')
        }
    }

    return (
        <section className="panel auth-grid page-enter">
            <form className="form" onSubmit={(e) => void onLogin(e)}>
                <h2>登录</h2>
                <label>
                    用户名
                    <input
                        required
                        value={authForm.username}
                        onChange={(e) => setAuthForm((prev) => ({ ...prev, username: e.target.value }))}
                    />
                </label>
                <label>
                    密码
                    <input
                        type="password"
                        required
                        value={authForm.password}
                        onChange={(e) => setAuthForm((prev) => ({ ...prev, password: e.target.value }))}
                    />
                </label>
                <button type="submit">登录</button>
            </form>

            <form className="form" onSubmit={(e) => void handleRegister(e)}>
                <h2>注册</h2>
                <label>
                    用户名
                    <input
                        required
                        value={authForm.username}
                        onChange={(e) => setAuthForm((prev) => ({ ...prev, username: e.target.value }))}
                    />
                </label>
                <label>
                    昵称
                    <input
                        value={authForm.nickname}
                        onChange={(e) => setAuthForm((prev) => ({ ...prev, nickname: e.target.value }))}
                    />
                </label>
                <label>
                    密码
                    <input
                        type="password"
                        required
                        value={authForm.password}
                        onChange={(e) => setAuthForm((prev) => ({ ...prev, password: e.target.value }))}
                    />
                </label>
                <button type="submit">注册</button>
            </form>
        </section>
    )
}
