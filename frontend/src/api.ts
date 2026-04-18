export type ApiResult<T> = {
    code: number
    message: string
    data?: T
}

export type UserProfile = {
    id: string
    username: string
    nickname: string
}

export type ArticleStat = {
    article_id: string
    view_count: number
    like_count: number
    collect_count: number
    comment_count: number
}

export type Article = {
    id: string
    user_id: string
    title: string
    summary: string
    content_md: string
    cover_img: string
    status: number
    is_top: boolean
    created_at: string
    updated_at: string
    published_at?: string
    stat?: ArticleStat
}

export type PageResult<T> = {
    list: T[]
    cursor?: string
    has_more: boolean
}

type RequestOptions = {
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
    token?: string
    body?: unknown
}

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://127.0.0.1:8080'

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers: Record<string, string> = {}
    if (options.body !== undefined) {
        headers['Content-Type'] = 'application/json'
    }
    if (options.token) {
        headers.Authorization = `Bearer ${options.token}`
    }

    const response = await fetch(`${API_BASE}${path}`, {
        method: options.method ?? 'GET',
        headers,
        credentials: 'include',
        body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
    })

    let payload: ApiResult<T>
    try {
        payload = (await response.json()) as ApiResult<T>
    } catch {
        throw new Error(`HTTP ${response.status}`)
    }

    if (!response.ok || payload.code !== 0) {
        throw new Error(payload.message || `HTTP ${response.status}`)
    }

    return payload.data as T
}

export function getApiBase(): string {
    return API_BASE
}

export function register(data: { username: string; password: string; nickname: string }) {
    return request<UserProfile>('/api/auth/register', {
        method: 'POST',
        body: data,
    })
}

export function login(data: { username: string; password: string }) {
    return request<{ access_token: string }>('/api/auth/login', {
        method: 'POST',
        body: data,
    })
}

export function logout(token: string) {
    return request<null>('/api/auth/logout', {
        method: 'POST',
        token,
    })
}

export function listArticles(cursor = '', limit = 20) {
    const query = new URLSearchParams({ limit: String(limit) })
    if (cursor) {
        query.set('cursor', cursor)
    }
    return request<PageResult<Article>>(`/api/articles?${query.toString()}`)
}

export function listAuthorArticles(uid: string, cursor = '', limit = 20) {
    const query = new URLSearchParams({ limit: String(limit) })
    if (cursor) {
        query.set('cursor', cursor)
    }
    return request<PageResult<Article>>(`/api/users/${uid}/articles?${query.toString()}`)
}

export function getArticle(id: string) {
    return request<Article>(`/api/articles/${id}`)
}

export function createArticle(
    token: string,
    data: {
        title: string
        summary: string
        content_md: string
        cover_img: string
        status: number
    },
) {
    return request<{ id: string; status: number }>('/api/articles', {
        method: 'POST',
        token,
        body: data,
    })
}

export function updateArticle(
    token: string,
    id: string,
    data: {
        title: string
        summary: string
        content_md: string
        cover_img: string
        status: number
    },
) {
    return request<null>(`/api/articles/${id}`, {
        method: 'PUT',
        token,
        body: data,
    })
}

export function deleteArticle(token: string, id: string) {
    return request<null>(`/api/articles/${id}`, {
        method: 'DELETE',
        token,
    })
}

export function hotRank(window = '24h', limit = 20) {
    const query = new URLSearchParams({ window, limit: String(limit) })
    return request<{ list: Article[] }>(`/api/rank/hot?${query.toString()}`)
}

export function searchArticles(q: string, limit = 20) {
    const query = new URLSearchParams({ q, limit: String(limit) })
    return request<{ list: Article[] }>(`/api/search?${query.toString()}`)
}

export function toggleLike(token: string, articleId: string) {
    return request<{ liked: boolean; count: number }>(`/api/articles/${articleId}/like`, {
        method: 'POST',
        token,
    })
}

export function toggleCollect(token: string, articleId: string, note = '') {
    return request<{ collected: boolean; count: number }>(`/api/articles/${articleId}/collect`, {
        method: 'POST',
        token,
        body: note ? { note } : {},
    })
}

export function followAuthor(token: string, authorId: string) {
    return request<null>(`/api/follow/${authorId}`, {
        method: 'POST',
        token,
    })
}

export function unfollowAuthor(token: string, authorId: string) {
    return request<null>(`/api/follow/${authorId}`, {
        method: 'DELETE',
        token,
    })
}

export function feedFollowing(token: string, cursor = '', limit = 20) {
    const query = new URLSearchParams({ limit: String(limit) })
    if (cursor) {
        query.set('cursor', cursor)
    }
    return request<PageResult<Article>>(`/api/feed/following?${query.toString()}`, {
        token,
    })
}
