import ReactMarkdown, { type Components } from 'react-markdown'
import remarkBreaks from 'remark-breaks'
import { Link } from 'react-router-dom'

// A small markdown subset for item descriptions and profiles. Raw HTML is never rendered, and
// anything outside the allowlist (headings, images, tables) is shown as plain text.
const allowedElements = ['p', 'br', 'strong', 'em', 'a', 'ul', 'ol', 'li', 'blockquote', 'code']

const hashtagPattern = /(^|[^\p{L}\p{N}_&/])#([\p{L}\p{N}_]+)/gu

type MdNode = {
  type: string
  value?: string
  url?: string
  children?: MdNode[]
  data?: Record<string, unknown>
}

// Turns #hashtags in text into links to Explore filtered on that tag. Matches the server's rule for tags.
function linkHashtags() {
  const visit = (node: MdNode) => {
    if (!node.children || node.type === 'link') return
    const next: MdNode[] = []
    for (const child of node.children) {
      if (child.type !== 'text' || child.value === undefined) {
        visit(child)
        next.push(child)
        continue
      }
      const text = child.value
      let last = 0
      for (const m of text.matchAll(hashtagPattern)) {
        const start = (m.index ?? 0) + m[1].length
        const tag = m[2].toLowerCase()
        if (start > last) next.push({ type: 'text', value: text.slice(last, start) })
        next.push({
          type: 'link',
          url: `/explore?tag=${encodeURIComponent(tag)}`,
          data: { hProperties: { className: 'hashtag' } },
          children: [{ type: 'text', value: `#${m[2]}` }],
        })
        last = (m.index ?? 0) + m[0].length
      }
      if (last < text.length) next.push({ type: 'text', value: text.slice(last) })
    }
    node.children = next
  }
  return (tree: MdNode) => visit(tree)
}

const linkBase = 'underline underline-offset-2 hover:text-indigo-500 dark:hover:text-indigo-300'

const components: Components = {
  a: ({ node: _node, href, className, children }) => {
    if (href?.startsWith('/')) {
      const isTag = className === 'hashtag'
      return (
        <Link
          to={href}
          className={isTag ? 'text-[0.9em] font-medium text-indigo-600 hover:underline dark:text-indigo-400' : `${linkBase} text-indigo-600 dark:text-indigo-400`}
        >
          {children}
        </Link>
      )
    }
    return (
      <a href={href} target="_blank" rel="noopener noreferrer nofollow" className={`${linkBase} text-indigo-600 dark:text-indigo-400`}>
        {children}
      </a>
    )
  },
}

export default function Markdown({ text, className = '' }: { text: string; className?: string }) {
  return (
    <div className={`space-y-3 break-words leading-relaxed ${className}`}>
      <ReactMarkdown
        allowedElements={allowedElements}
        unwrapDisallowed
        remarkPlugins={[remarkBreaks, linkHashtags]}
        components={components}
      >
        {text}
      </ReactMarkdown>
    </div>
  )
}
