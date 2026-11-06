import { Link } from 'react-router-dom'
import { usePopularTags } from '../hooks'
import { chip } from '../ui'

// The most-used public hashtags. Each one opens Explore filtered to that tag.
export default function TopTags() {
  const tags = usePopularTags()
  if (!tags.data || tags.data.length === 0) return null
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-sm font-medium text-stone-500 dark:text-stone-400">Top tags</span>
      {tags.data.map((t) => (
        <Link
          key={t.tag}
          to={`/explore?tag=${encodeURIComponent(t.tag)}`}
          className={`${chip} transition hover:bg-indigo-100 dark:hover:bg-indigo-900`}
          title={`${t.count} public ${t.count === 1 ? 'item' : 'items'}`}
        >
          #{t.tag}
        </Link>
      ))}
    </div>
  )
}
