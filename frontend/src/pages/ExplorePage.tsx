import ArticleCard from '../components/ArticleCard'
import { useForum } from '../app/useForum'

export default function ExplorePage() {
    const {
        articles,
        hotList,
        searchInput,
        searchMode,
        setSearchInput,
        handleSearch,
        loadExplore,
        handleOpenArticle,
        handleToggleLike,
        handleToggleCollect,
    } = useForum()

    return (
        <main className="layout page-enter">
            <section className="panel">
                <div className="panel-head">
                    <h2>{searchMode ? '搜索结果' : '最新文章'}</h2>
                    <form className="search" onSubmit={(e) => void handleSearch(e)}>
                        <input
                            value={searchInput}
                            onChange={(e) => setSearchInput(e.target.value)}
                            placeholder="输入关键词，回车检索"
                        />
                        <button type="submit">搜索</button>
                    </form>
                </div>

                <div className="card-grid">
                    {articles.length ? (
                        articles.map((article) => (
                            <ArticleCard
                                key={article.id}
                                article={article}
                                onOpen={(id) => void handleOpenArticle(id)}
                                onLike={(id) => void handleToggleLike(id)}
                                onCollect={(id) => void handleToggleCollect(id)}
                            />
                        ))
                    ) : (
                        <p>暂无数据。</p>
                    )}
                </div>
            </section>

            <aside className="panel side">
                <div className="panel-head">
                    <h2>热榜</h2>
                    <button onClick={() => void loadExplore()}>刷新</button>
                </div>
                <ol className="rank-list">
                    {hotList.map((item) => (
                        <li key={item.id}>
                            <button className="link" onClick={() => void handleOpenArticle(item.id)}>
                                {item.title}
                            </button>
                        </li>
                    ))}
                </ol>
            </aside>
        </main>
    )
}
