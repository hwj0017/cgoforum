import { useForum } from '../app/useForum'
import { formatTime } from '../utils/format'

export default function ArticleDetailModal() {
    const { selected, setSelected, detailRef, handleToggleLike, handleToggleCollect, handleFollow } = useForum()

    if (!selected) {
        return null
    }

    return (
        <section className="detail-modal" aria-modal="true" role="dialog">
            <button className="detail-backdrop" onClick={() => setSelected(null)} aria-label="关闭详情" />
            <div className="panel detail detail-card" ref={detailRef}>
                <div className="panel-head">
                    <h2>文章详情 #{selected.id}</h2>
                    <button onClick={() => setSelected(null)}>关闭</button>
                </div>
                <h3>{selected.title}</h3>
                <p className="summary">{selected.summary}</p>
                <pre>{selected.content_md}</pre>
                <div className="meta">
                    <span>作者: {selected.user_id}</span>
                    <span>发布时间: {formatTime(selected.published_at || selected.created_at)}</span>
                    <span>点赞: {selected.stat?.like_count ?? '-'}</span>
                    <span>收藏: {selected.stat?.collect_count ?? '-'}</span>
                </div>
                <div className="actions">
                    <button onClick={() => void handleToggleLike(selected.id)}>点赞切换</button>
                    <button onClick={() => void handleToggleCollect(selected.id)}>收藏切换</button>
                    <button onClick={() => void handleFollow(selected.user_id)}>关注作者</button>
                </div>
            </div>
        </section>
    )
}
