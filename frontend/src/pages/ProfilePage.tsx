import { Link, useParams } from 'react-router-dom'
import { Compass, Settings } from 'lucide-react'
import ItemCard from '../components/ItemCard'
import Markdown from '../components/Markdown'
import { EmptyState, ErrorState, Loading } from '../components/Status'
import { useMe, usePublicUser, useUserItems } from '../hooks'
import { btnSecondary } from '../ui'

export default function ProfilePage() {
  const { id = '' } = useParams()
  const profile = usePublicUser(id)
  const items = useUserItems(id)
  const me = useMe()

  if (profile.isPending) return <Loading />
  if (profile.error) return <ErrorState message={profile.error.message || 'User not found.'} />

  const user = profile.data
  const isMe = me.data?.id === user.id

  return (
    <div className="space-y-8">
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

      <section className="space-y-4">
        <h2 className="text-lg font-semibold">Public items</h2>
        {items.isPending && <Loading />}
        {items.isError && <ErrorState message={items.error.message} />}
        {items.data &&
          (items.data.length === 0 ? (
            <EmptyState icon={<Compass size={40} />} title="Nothing public yet">
              {isMe ? 'Items you mark as public will show up here.' : 'This collector has no public items yet.'}
            </EmptyState>
          ) : (
            <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {items.data.map((item) => (
                <ItemCard key={item.id} item={item} />
              ))}
            </div>
          ))}
      </section>
    </div>
  )
}
