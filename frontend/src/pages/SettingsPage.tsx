import { useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { UserRound } from 'lucide-react'
import { api } from '../api'
import VisibilityToggle from '../components/VisibilityToggle'
import { ErrorState, Loading } from '../components/Status'
import { useMe } from '../hooks'
import type { User, Visibility } from '../types'
import { btnPrimary, card, input, label } from '../ui'

const MAX_PROFILE = 2000

export default function SettingsPage() {
  const me = useMe()
  if (me.isPending) return <Loading />
  if (!me.data) return <ErrorState message="Sign in to change your settings." />
  return <SettingsForm key={me.data.id} user={me.data} />
}

function SettingsForm({ user }: { user: User }) {
  const qc = useQueryClient()
  const [profile, setProfile] = useState(user.profile)
  const [visibility, setVisibility] = useState<Visibility>(user.defaultVisibility)
  const [saved, setSaved] = useState(false)

  const save = useMutation({
    mutationFn: () => api.updateMe({ profile, defaultVisibility: visibility }),
    onSuccess: (updated) => {
      qc.setQueryData(['me'], updated)
      setSaved(true)
    },
  })

  const dirty = profile !== user.profile || visibility !== user.defaultVisibility

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    setSaved(false)
    save.mutate()
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-bold tracking-tight sm:text-3xl">
          <UserRound size={26} className="text-indigo-600 dark:text-indigo-400" /> Settings
        </h1>
        <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
          Signed in as {user.name}.{' '}
          <Link to={`/users/${user.id}`} className="font-medium text-indigo-600 hover:underline dark:text-indigo-400">
            View your public profile
          </Link>
        </p>
      </div>

      <form onSubmit={onSubmit} className={`${card} space-y-7 p-6 sm:p-8`}>
        <div>
          <label htmlFor="profile" className={label}>
            Profile
          </label>
          <textarea
            id="profile"
            className={`${input} min-h-32 resize-y`}
            rows={5}
            maxLength={MAX_PROFILE}
            placeholder="Tell other collectors about yourself…"
            value={profile}
            onChange={(e) => {
              setProfile(e.target.value)
              setSaved(false)
            }}
          />
          <p className="mt-1.5 text-xs text-stone-500 dark:text-stone-400">
            {profile.length} / {MAX_PROFILE}. Shown on your public profile.
          </p>
        </div>

        <div>
          <span className={label}>Default visibility</span>
          <VisibilityToggle
            value={visibility}
            onChange={(v) => {
              setVisibility(v)
              setSaved(false)
            }}
          />
          <p className="mt-1.5 text-xs text-stone-500 dark:text-stone-400">
            New items and folders start with this setting. You can still change each one.
          </p>
        </div>

        {save.error && (
          <p role="alert" className="text-sm text-rose-600 dark:text-rose-400">
            {save.error.message}
          </p>
        )}

        <div className="flex items-center justify-end gap-3">
          {saved && <span className="text-sm text-emerald-600 dark:text-emerald-400">Saved</span>}
          <button type="submit" disabled={save.isPending || !dirty} className={btnPrimary}>
            {save.isPending ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </div>
  )
}
