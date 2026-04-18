import { createContext, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Dispatch, FormEvent, ReactNode, RefObject, SetStateAction } from 'react'
import {
    createArticle,
    deleteArticle,
    feedFollowing,
    followAuthor,
    getArticle,
    hotRank,
    listArticles,
    login,
    logout,
    register,
    searchArticles,
    toggleCollect,
    toggleLike,
    unfollowAuthor,
    updateArticle,
} from '../api'
import type { Article } from '../api'

type Message = {
    type: 'success' | 'error' | 'info'
    text: string
}

type AuthForm = {
    username: string
    password: string
    nickname: string
}

type ArticleForm = {
    id: string
    title: string
    summary: string
    content_md: string
    cover_img: string
    status: number
}

export type ForumContextValue = {
    token: string
    hasToken: boolean
    message: Message | null
    loading: boolean
    selected: Article | null
    detailRef: RefObject<HTMLDivElement | null>
    articles: Article[]
    hotList: Article[]
    followingFeed: Article[]
    searchInput: string
    searchMode: boolean
    authForm: AuthForm
    articleForm: ArticleForm
    setSearchInput: (value: string) => void
    setSelected: (article: Article | null) => void
    setAuthForm: Dispatch<SetStateAction<AuthForm>>
    setArticleForm: Dispatch<SetStateAction<ArticleForm>>
    loadExplore: () => Promise<void>
    loadFollowingFeed: () => Promise<void>
    handleSearch: (event: FormEvent<HTMLFormElement>) => Promise<void>
    handleOpenArticle: (articleId: string) => Promise<void>
    handleLogin: (event: FormEvent<HTMLFormElement>) => Promise<boolean>
    handleRegister: (event: FormEvent<HTMLFormElement>) => Promise<void>
    handleLogout: () => Promise<void>
    handleSubmitArticle: (event: FormEvent<HTMLFormElement>) => Promise<void>
    handleDeleteArticle: () => Promise<void>
    handleToggleLike: (articleId: string) => Promise<void>
    handleToggleCollect: (articleId: string) => Promise<void>
    handleFollow: (authorId: string, unfollow?: boolean) => Promise<void>
}

const TOKEN_KEY = 'cgoforum_access_token'

const defaultArticleForm: ArticleForm = {
    id: '',
    title: '',
    summary: '',
    content_md: '',
    cover_img: '',
    status: 1,
}

const ForumContext = createContext<ForumContextValue | null>(null)

export function ForumProvider({ children }: { children: ReactNode }) {
    const [token, setToken] = useState<string>(() => localStorage.getItem(TOKEN_KEY) ?? '')
    const [message, setMessage] = useState<Message | null>(null)
    const [loading, setLoading] = useState(false)
    const [selected, setSelected] = useState<Article | null>(null)

    const [articles, setArticles] = useState<Article[]>([])
    const [hotList, setHotList] = useState<Article[]>([])
    const [followingFeed, setFollowingFeed] = useState<Article[]>([])

    const [searchInput, setSearchInputState] = useState('')
    const [searchMode, setSearchMode] = useState(false)
    const detailRef = useRef<HTMLDivElement>(null)

    const [authForm, setAuthForm] = useState<AuthForm>({
        username: '',
        password: '',
        nickname: '',
    })

    const [articleForm, setArticleForm] = useState<ArticleForm>(defaultArticleForm)

    useEffect(() => {
        localStorage.setItem(TOKEN_KEY, token)
    }, [token])

    const hasToken = useMemo(() => Boolean(token), [token])

    const withBusy = useCallback(async <T,>(task: () => Promise<T>): Promise<T | null> => {
        setLoading(true)
        setMessage(null)
        try {
            return await task()
        } catch (error) {
            const text = error instanceof Error ? error.message : '请求失败'
            setMessage({ type: 'error', text })
            return null
        } finally {
            setLoading(false)
        }
    }, [])

    const loadExplore = useCallback(async () => {
        const articlesRes = await withBusy(() => listArticles('', 20))
        const hotRes = await withBusy(() => hotRank('24h', 10))
        if (articlesRes) {
            setArticles(articlesRes.list)
        }
        if (hotRes) {
            setHotList(hotRes.list)
        }
    }, [withBusy])

    const loadFollowingFeed = useCallback(async () => {
        if (!token) {
            setMessage({ type: 'info', text: '请先登录再查看关注流。' })
            return
        }
        const feed = await withBusy(() => feedFollowing(token, '', 20))
        if (feed) {
            setFollowingFeed(feed.list)
        }
    }, [token, withBusy])

    useEffect(() => {
        void loadExplore()
    }, [loadExplore])

    useEffect(() => {
        if (!selected || !detailRef.current) {
            return
        }
        detailRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }, [selected])

    const setSearchInput = useCallback((value: string) => {
        setSearchInputState(value)
    }, [])

    const handleSearch = useCallback(async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        const q = searchInput.trim()
        if (!q) {
            setSearchMode(false)
            await loadExplore()
            return
        }
        const res = await withBusy(() => searchArticles(q, 20))
        if (res) {
            setSearchMode(true)
            setArticles(res.list)
            setMessage({ type: 'info', text: `检索到 ${res.list.length} 条结果。` })
        }
    }, [searchInput, loadExplore, withBusy])

    const handleOpenArticle = useCallback(async (articleId: string) => {
        const detail = await withBusy(() => getArticle(articleId))
        if (detail) {
            setSelected(detail)
        }
    }, [withBusy])

    const handleLogin = useCallback(async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        const res = await withBusy(() =>
            login({ username: authForm.username.trim(), password: authForm.password }),
        )
        if (!res) {
            return false
        }
        setToken(res.access_token)
        setMessage({ type: 'success', text: '登录成功。' })
        return true
    }, [authForm.password, authForm.username, withBusy])

    const handleRegister = useCallback(async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        const res = await withBusy(() =>
            register({
                username: authForm.username.trim(),
                password: authForm.password,
                nickname: authForm.nickname.trim() || authForm.username.trim(),
            }),
        )
        if (!res) {
            return
        }
        setMessage({ type: 'success', text: `注册成功，用户 ID: ${res.id}` })
    }, [authForm.nickname, authForm.password, authForm.username, withBusy])

    const handleLogout = useCallback(async () => {
        if (token) {
            await withBusy(() => logout(token))
        }
        setToken('')
        setSelected(null)
        setMessage({ type: 'success', text: '已退出登录。' })
    }, [token, withBusy])

    const handleSubmitArticle = useCallback(async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault()
        if (!token) {
            setMessage({ type: 'error', text: '请先登录后再发布。' })
            return
        }

        const payload = {
            title: articleForm.title.trim(),
            summary: articleForm.summary.trim(),
            content_md: articleForm.content_md,
            cover_img: articleForm.cover_img.trim(),
            status: articleForm.status,
        }

        if (!payload.title || !payload.content_md) {
            setMessage({ type: 'error', text: '标题和正文不能为空。' })
            return
        }

        if (!articleForm.id.trim()) {
            const created = await withBusy(() => createArticle(token, payload))
            if (!created) {
                return
            }
            setMessage({ type: 'success', text: `发布成功，文章 ID: ${created.id}` })
        } else {
            const id = articleForm.id.trim()
            if (!/^\d+$/.test(id)) {
                setMessage({ type: 'error', text: '更新时请输入有效文章 ID。' })
                return
            }
            const ok = await withBusy(() => updateArticle(token, id, payload))
            if (ok === null) {
                return
            }
            setMessage({ type: 'success', text: `文章 ${id} 更新成功。` })
        }

        setArticleForm(defaultArticleForm)
        setSearchMode(false)
        await loadExplore()
    }, [articleForm, token, withBusy, loadExplore])

    const handleDeleteArticle = useCallback(async () => {
        if (!token) {
            setMessage({ type: 'error', text: '请先登录。' })
            return
        }
        const id = articleForm.id.trim()
        if (!/^\d+$/.test(id)) {
            setMessage({ type: 'error', text: '请输入要删除的文章 ID。' })
            return
        }
        const ok = await withBusy(() => deleteArticle(token, id))
        if (ok === null) {
            return
        }
        setMessage({ type: 'success', text: `文章 ${id} 已删除。` })
        await loadExplore()
    }, [token, articleForm.id, withBusy, loadExplore])

    const handleToggleLike = useCallback(async (articleId: string) => {
        if (!token) {
            setMessage({ type: 'error', text: '请先登录后点赞。' })
            return
        }
        const res = await withBusy(() => toggleLike(token, articleId))
        if (!res) {
            return
        }

        setMessage({
            type: 'success',
            text: res.liked ? `点赞成功，当前 ${res.count}` : `已取消点赞，当前 ${res.count}`,
        })

        await Promise.all([loadExplore(), handleOpenArticle(articleId)])
    }, [token, withBusy, loadExplore, handleOpenArticle])

    const handleToggleCollect = useCallback(async (articleId: string) => {
        if (!token) {
            setMessage({ type: 'error', text: '请先登录后收藏。' })
            return
        }
        const res = await withBusy(() => toggleCollect(token, articleId, '前端快捷收藏'))
        if (!res) {
            return
        }

        setMessage({
            type: 'success',
            text: res.collected ? `收藏成功，当前 ${res.count}` : `已取消收藏，当前 ${res.count}`,
        })

        await Promise.all([loadExplore(), handleOpenArticle(articleId)])
    }, [token, withBusy, loadExplore, handleOpenArticle])

    const handleFollow = useCallback(async (authorId: string, unfollow = false) => {
        if (!token) {
            setMessage({ type: 'error', text: '请先登录后关注。' })
            return
        }
        const action = unfollow ? unfollowAuthor : followAuthor
        const ok = await withBusy(() => action(token, authorId))
        if (ok === null) {
            return
        }
        setMessage({ type: 'success', text: unfollow ? '已取消关注。' : '关注成功。' })
    }, [token, withBusy])

    const value = useMemo<ForumContextValue>(
        () => ({
            token,
            hasToken,
            message,
            loading,
            selected,
            detailRef,
            articles,
            hotList,
            followingFeed,
            searchInput,
            searchMode,
            authForm,
            articleForm,
            setSearchInput,
            setSelected,
            setAuthForm,
            setArticleForm,
            loadExplore,
            loadFollowingFeed,
            handleSearch,
            handleOpenArticle,
            handleLogin,
            handleRegister,
            handleLogout,
            handleSubmitArticle,
            handleDeleteArticle,
            handleToggleLike,
            handleToggleCollect,
            handleFollow,
        }),
        [
            token,
            hasToken,
            message,
            loading,
            selected,
            articles,
            hotList,
            followingFeed,
            searchInput,
            searchMode,
            authForm,
            articleForm,
            setSearchInput,
            loadExplore,
            loadFollowingFeed,
            handleSearch,
            handleOpenArticle,
            handleLogin,
            handleRegister,
            handleLogout,
            handleSubmitArticle,
            handleDeleteArticle,
            handleToggleLike,
            handleToggleCollect,
            handleFollow,
        ],
    )

    return <ForumContext.Provider value={value}>{children}</ForumContext.Provider>
}

export { ForumContext }
