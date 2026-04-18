import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useForum } from '../app/useForum'

export default function ComposePage() {
    const { articleForm, setArticleForm, handleSubmitArticle, handleDeleteArticle } = useForum()
    const navigate = useNavigate()

    async function onSubmit(event: FormEvent<HTMLFormElement>) {
        await handleSubmitArticle(event)
        navigate('/')
    }

    return (
        <section className="panel page-enter">
            <h2>创作中心</h2>
            <form className="form" onSubmit={(e) => void onSubmit(e)}>
                <label>
                    文章 ID（留空=新建）
                    <input
                        value={articleForm.id}
                        onChange={(e) => setArticleForm((prev) => ({ ...prev, id: e.target.value }))}
                        placeholder="例如 1001"
                    />
                </label>
                <label>
                    标题
                    <input
                        required
                        value={articleForm.title}
                        onChange={(e) => setArticleForm((prev) => ({ ...prev, title: e.target.value }))}
                    />
                </label>
                <label>
                    摘要
                    <input
                        value={articleForm.summary}
                        onChange={(e) => setArticleForm((prev) => ({ ...prev, summary: e.target.value }))}
                    />
                </label>
                <label>
                    封面 URL
                    <input
                        value={articleForm.cover_img}
                        onChange={(e) => setArticleForm((prev) => ({ ...prev, cover_img: e.target.value }))}
                    />
                </label>
                <label>
                    状态
                    <select
                        value={articleForm.status}
                        onChange={(e) =>
                            setArticleForm((prev) => ({
                                ...prev,
                                status: Number(e.target.value),
                            }))
                        }
                    >
                        <option value={0}>草稿</option>
                        <option value={1}>发布</option>
                    </select>
                </label>
                <label>
                    正文（Markdown）
                    <textarea
                        required
                        rows={10}
                        value={articleForm.content_md}
                        onChange={(e) => setArticleForm((prev) => ({ ...prev, content_md: e.target.value }))}
                    />
                </label>
                <div className="actions row">
                    <button type="submit">提交</button>
                    <button type="button" onClick={() => void handleDeleteArticle()}>
                        按 ID 删除
                    </button>
                </div>
            </form>
        </section>
    )
}
