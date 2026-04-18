import { useEffect } from 'react'
import ArticleCard from '../components/ArticleCard'
import { useForum } from '../app/useForum'

export default function FeedPage() {
    const { followingFeed, loadFollowingFeed, handleOpenArticle, handleFollow } = useForum()

    useEffect(() => {
        void loadFollowingFeed()
    }, [loadFollowingFeed])

    return (
        <section className="panel page-enter">
            <div className="panel-head">
                <h2>关注流</h2>
                <button onClick={() => void loadFollowingFeed()}>刷新</button>
            </div>

            <div className="card-grid">
                {followingFeed.length ? (
                    followingFeed.map((article) => (
                        <ArticleCard
                            key={article.id}
                            article={article}
                            onOpen={(id) => void handleOpenArticle(id)}
                            onUnfollow={(authorId) => void handleFollow(authorId, true)}
                        />
                    ))
                ) : (
                    <p>暂无关注流数据。</p>
                )}
            </div>
        </section>
    )
}
