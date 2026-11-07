import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { MessageSquare, Pencil } from 'lucide-react'
import { api } from '../api'
import { useComments, useMe } from '../hooks'
import type { Item, ItemComment } from '../types'
import { btnGhost, btnPrimary, btnSecondary, card } from '../ui'
import Markdown from './Markdown'
import MarkdownField from './MarkdownField'

const MAX_COMMENT_LENGTH = 5000

function when(value: string) {
  return new Date(value).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

function edited(comment: ItemComment) {
  return new Date(comment.updatedAt).getTime() !== new Date(comment.createdAt).getTime()
}

function CommentAvatar({ url, name }: { url: string; name: string }) {
  if (url) {
    return <img src={url} alt="" referrerPolicy="no-referrer" className="h-8 w-8 shrink-0 rounded-full" />
  }
  return (
    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-stone-200 text-xs font-semibold dark:bg-stone-700">
      {name.trim().charAt(0).toUpperCase() || '?'}
    </span>
  )
}

// Comments are only rendered for signed-in users. Comments are owner-only for now,
// but the API already carries a visibility value so this can become a per-comment
// control without changing the data model.
export default function Comments({ item }: { item: Item }) {
  const { data: me } = useMe()
  const qc = useQueryClient()
  const comments = useComments(item.id, !!me)
  const [draft, setDraft] = useState('')
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editDraft, setEditDraft] = useState('')
  const [error, setError] = useState('')

  const refresh = () => qc.invalidateQueries({ queryKey: ['comments', item.id] })

  const create = useMutation({
    mutationFn: () => api.createComment(item.id, { body: draft }),
    onSuccess: () => {
      setDraft('')
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  const update = useMutation({
    mutationFn: (commentID: string) => api.updateComment(item.id, commentID, { body: editDraft }),
    onSuccess: () => {
      setEditingID(null)
      setEditDraft('')
      setError('')
      refresh()
    },
    onError: (err) => setError(err.message),
  })

  if (!me) return null

  const isOwner = me.id === item.ownerId
  const list = comments.data ?? []

  const startEditing = (comment: ItemComment) => {
    setEditingID(comment.id)
    setEditDraft(comment.body)
    setError('')
  }

  return (
    <section aria-labelledby="comments-heading" className={`${card} p-5 sm:p-6`}>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <h2 id="comments-heading" className="flex items-center gap-2 text-lg font-semibold">
          <MessageSquare size={18} className="text-stone-400" />
          Comments
        </h2>
        <span className="text-xs text-stone-500 dark:text-stone-400">Only signed-in people can see these</span>
      </div>

      {comments.isPending && <p className="mt-4 text-sm text-stone-500 dark:text-stone-400">Loading comments…</p>}
      {comments.error && (
        <p role="alert" className="mt-4 text-sm text-rose-600 dark:text-rose-400">
          {comments.error.message}
        </p>
      )}
      {comments.isSuccess && list.length === 0 && (
        <p className="mt-4 text-sm text-stone-500 dark:text-stone-400">No comments yet.</p>
      )}

      {list.length > 0 && (
        <ul className="mt-4 space-y-3">
          {list.map((comment) => (
            <li key={comment.id} className="rounded-xl border border-stone-200 p-4 dark:border-stone-800">
              <div className="flex items-start gap-3">
                <CommentAvatar url={comment.authorAvatarUrl} name={comment.authorName} />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm">
                      <span className="font-medium">{comment.authorName}</span>
                      <time dateTime={comment.createdAt} className="text-xs text-stone-500 dark:text-stone-400">
                        {when(comment.createdAt)}
                      </time>
                      {edited(comment) && (
                        <span className="text-xs text-stone-400 dark:text-stone-500" title={`Edited ${when(comment.updatedAt)}`}>
                          edited
                        </span>
                      )}
                    </div>
                    {comment.authorId === me.id && editingID !== comment.id && (
                      <button
                        type="button"
                        onClick={() => startEditing(comment)}
                        className={btnGhost}
                        aria-label={`Edit comment from ${comment.authorName}`}
                      >
                        <Pencil size={14} />
                      </button>
                    )}
                  </div>

                  {editingID === comment.id ? (
                    <div className="mt-3 space-y-3">
                      <MarkdownField
                        id={`edit-comment-${comment.id}`}
                        labelText="Edit comment"
                        value={editDraft}
                        onChange={setEditDraft}
                        maxLength={MAX_COMMENT_LENGTH}
                        rows={3}
                      />
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          onClick={() => update.mutate(comment.id)}
                          disabled={update.isPending || !editDraft.trim()}
                          className={btnPrimary}
                        >
                          {update.isPending ? 'Saving…' : 'Save'}
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setEditingID(null)
                            setEditDraft('')
                            setError('')
                          }}
                          disabled={update.isPending}
                          className={btnSecondary}
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <Markdown text={comment.body} className="mt-1 text-sm text-stone-800 dark:text-stone-200" />
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {isOwner && (
        <form
          onSubmit={(e) => {
            e.preventDefault()
            create.mutate()
          }}
          className={`${list.length > 0 ? 'mt-5 border-t border-stone-200 pt-5 dark:border-stone-800' : 'mt-4'} space-y-3`}
        >
          <MarkdownField
            id="new-comment"
            labelText="Add a comment"
            value={draft}
            onChange={setDraft}
            maxLength={MAX_COMMENT_LENGTH}
            rows={3}
            placeholder="Notes for yourself, or context you want to share with signed-in visitors…"
            help="Basic markdown works, just like item descriptions."
          />
          <button type="submit" disabled={create.isPending || !draft.trim()} className={btnPrimary}>
            {create.isPending ? 'Posting…' : 'Post comment'}
          </button>
        </form>
      )}

      {!isOwner && (
        <p className="mt-4 text-xs text-stone-500 dark:text-stone-400">
          Only the item's owner can add comments for now.
        </p>
      )}

      {error && (
        <p role="alert" className="mt-3 text-sm text-rose-600 dark:text-rose-400">
          {error}
        </p>
      )}
    </section>
  )
}
