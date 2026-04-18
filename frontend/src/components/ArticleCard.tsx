import type { Article } from '../api'
import { formatTime, shortText } from '../utils/format'

type Props = {
    article: Article
    onOpen: (id: string) => void
    onLike?: (id: string) => void
    onCollect?: (id: string) => void
    onUnfollow?: (authorId: string) => void
}

export default function ArticleCard({ article, onOpen, onLike, onCollect, onUnfollow }: Props) {
    return (
        <article className="card">
            <div className="card-head">
                <h3>{article.title}</h3>
                <span className="pill">#{article.id}</span>
            </div>
            <p>{shortText(article.summary || article.content_md, 140)}</p>
            <div className="meta">
                <span>作者 {article.user_id}</span>
                <span>发布于 {formatTime(article.published_at || article.created_at)}</span>
            </div>
            <div className="actions">
                <button onClick={() => onOpen(article.id)}>详情</button>
                {onLike ? <button onClick={() => onLike(article.id)}>点赞</button> : null}
                {onCollect ? <button onClick={() => onCollect(article.id)}>收藏</button> : null}
                {onUnfollow ? <button onClick={() => onUnfollow(article.user_id)}>取消关注</button> : null}
            </div>
        </article>
    )
}
