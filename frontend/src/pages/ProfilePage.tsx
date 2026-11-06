import { Link, useParams, useSearchParams } from 'react-router-dom'
import { ChevronRight, Compass, Folder, Settings } from 'lucide-react'
import FolderHeader from '../components/FolderHeader'
import ItemBrowser from '../components/ItemBrowser'
import Markdown from '../components/Markdown'
import { EmptyState, ErrorState, Loading } from '../components/Status'
import { childrenOf, pathTo, rootFolder } from '../folders'
import { useMe, usePublicCollections, usePublicUser } from '../hooks'
import type { Collection, PublicUser } from '../types'
import { btnSecondary, card } from '../ui'

export default function ProfilePage() {
  const { id = '' } = useParams()
  const profile = usePublicUser(id)
  const folders = usePublicCollections(id)
  const me = useMe()

  if (profile.isPending || folders.isPending) return <Loading />
  if (profile.error) return <ErrorState message={profile.error.message || 'User not found.'} />
  if (folders.error) return <ErrorState message={folders.error.message} />

  const user = profile.data
  const isMe = me.data?.id === user.id
  const root = rootFolder(folders.data)

  return (
    <div className="space-y-8">
      <Header user={user} isMe={isMe} />
      {root ? (
        <FolderView userId={user.id} folders={folders.data} root={root} ownerName={user.name} />
      ) : (
        <EmptyState icon={<Compass size={40} />} title="Nothing public yet">
          {isMe ? 'Items you mark as public will show up here.' : 'This collector has no public items yet.'}
        </EmptyState>
      )}
    </div>
  )
}

function Header({ user, isMe }: { user: PublicUser; isMe: boolean }) {
  return (
    <header className="flex flex-wrap items-start gap-5">
      {user.avatarUrl ? (
        <img src={user.avatarUrl} alt="" className="h-20 w-20 rounded-full" referrerPolicy="no-referrer" />
      ) : (
        <span className="flex h-20 w-20 items-center justify-center rounded-full bg-stone-200 text-2xl font-semibold dark:bg-stone-700">
          {user.name.charAt(0).toUpperCase()}
        </span>
      )}
      <div className="min-w-0 flex-1">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{user.name}</h1>
        {user.profile ? (
          <Markdown text={user.profile} className="mt-2 text-stone-700 dark:text-stone-300" />
        ) : (
          <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">
            {isMe ? 'Add a profile so other collectors know who you are.' : 'No profile yet.'}
          </p>
        )}
      </div>
      {isMe && (
        <Link to="/settings" className={btnSecondary}>
          <Settings size={16} /> Edit settings
        </Link>
      )}
    </header>
  )
}

// The public part of the owner's folder tree, browsed with the same toolbar as the owner's own collection.
function FolderView({
  userId,
  folders,
  root,
  ownerName,
}: {
  userId: string
  folders: Collection[]
  root: Collection
  ownerName: string
}) {
  const [params, setParams] = useSearchParams()
  const current = folders.find((c) => c.id === params.get('folder')) ?? root
  const path = pathTo(folders, current.id)
  const subfolders = childrenOf(folders, current.id)
  const go = (id: string) => setParams(id === root.id ? {} : { folder: id })
  const nameOf = (f: Collection) => (f.parentId === null ? `${ownerName}'s collection` : f.name)

  return (
    <ItemBrowser
      scope="public"
      owner={userId}
      title={nameOf(current)}
      collectionId={current.id}
      before={
        <div className="space-y-4">
          <nav aria-label="Folder path" className="flex flex-wrap items-center gap-1 text-sm text-stone-500 dark:text-stone-400">
            {path.map((f, i) => (
              <span key={f.id} className="flex items-center gap-1">
                {i > 0 && <ChevronRight size={14} />}
                {i === path.length - 1 ? (
                  <span className="font-medium text-stone-800 dark:text-stone-200">{nameOf(f)}</span>
                ) : (
                  <button type="button" onClick={() => go(f.id)} className="cursor-pointer hover:text-indigo-600 dark:hover:text-indigo-400">
                    {nameOf(f)}
                  </button>
                )}
              </span>
            ))}
          </nav>
          {current.parentId !== null && <FolderHeader folder={current} />}
        </div>
      }
      folders={() =>
        subfolders.length > 0 && (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {subfolders.map((f) => (
              <button
                key={f.id}
                type="button"
                onClick={() => go(f.id)}
                className={`${card} flex cursor-pointer items-center gap-3 p-4 text-left transition hover:-translate-y-0.5 hover:shadow-md`}
              >
                <Folder size={20} className="shrink-0 text-indigo-500" />
                <span className="block truncate font-medium">{f.name}</span>
              </button>
            ))}
          </div>
        )
      }
    />
  )
}
