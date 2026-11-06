import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { Compass, Globe, Lock, Share2, Sparkles, Star, Tags } from 'lucide-react'
import { api } from '../api'
import { btnSecondary, card } from '../ui'

const providerLabels: Record<string, string> = {
  google: 'Continue with Google',
  github: 'Continue with GitHub',
}

const errorMessages: Record<string, string> = {
  invalid_state: 'Your sign-in session expired. Please try again.',
  denied: 'Sign-in was canceled.',
  exchange_failed: 'The provider rejected the sign-in. Please try again.',
  profile_failed: 'We could not read your profile from the provider.',
  server_error: 'Something went wrong on our side. Please try again.',
}

const features = [
  { icon: Tags, text: 'Tag everything you collect with #hashtags' },
  { icon: Star, text: 'Rate items and keep notes on what you love' },
  { icon: Globe, text: 'Toggle items between public and private' },
  { icon: Share2, text: 'Share a public link with anyone' },
]

export default function LoginPage() {
  const [params] = useSearchParams()
  const providers = useQuery({ queryKey: ['providers'], queryFn: api.providers })
  const error = params.get('error')

  return (
    <div className="mx-auto grid max-w-5xl items-center gap-12 py-8 md:grid-cols-2 md:py-16">
      <div className="space-y-6">
        <span className="inline-flex h-12 w-12 items-center justify-center rounded-2xl bg-indigo-600 text-white shadow-lg shadow-indigo-600/30">
          <Sparkles size={22} />
        </span>
        <h1 className="text-4xl font-bold tracking-tight md:text-5xl">
          Your collection, <span className="text-indigo-600 dark:text-indigo-400">beautifully cataloged.</span>
        </h1>
        <p className="text-lg text-stone-600 dark:text-stone-400">
          Consoles, albums, t-shirts, and everything else you love. Capture it, tag it, and show it off.
        </p>
        <ul className="space-y-3">
          {features.map(({ icon: Icon, text }) => (
            <li key={text} className="flex items-center gap-3 text-sm text-stone-700 dark:text-stone-300">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600 dark:bg-indigo-950 dark:text-indigo-400">
                <Icon size={16} />
              </span>
              {text}
            </li>
          ))}
        </ul>
      </div>

      <div className={`${card} space-y-5 p-8`}>
        <div className="space-y-1 text-center">
          <h2 className="text-xl font-semibold">Sign in</h2>
          <p className="text-sm text-stone-500 dark:text-stone-400">Use your existing account. No passwords to remember.</p>
        </div>

        {error && (
          <p role="alert" className="rounded-lg bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:bg-rose-950 dark:text-rose-300">
            {errorMessages[error] ?? 'Sign-in failed. Please try again.'}
          </p>
        )}

        <div className="flex flex-col gap-3">
          {providers.data?.providers.map((p) => (
            <a key={p} href={`/api/auth/${p}/login`} className={`${btnSecondary} w-full py-2.5`}>
              {providerLabels[p] ?? `Continue with ${p}`}
            </a>
          ))}
          {providers.isSuccess && providers.data.providers.length === 0 && (
            <p className="text-center text-sm text-stone-500">No sign-in providers are configured on this server.</p>
          )}
          {providers.isError && <p className="text-center text-sm text-rose-600">Could not reach the server.</p>}
        </div>

        <Link to="/explore" className="flex items-center justify-center gap-2 text-sm text-stone-500 hover:text-indigo-600 dark:text-stone-400">
          <Compass size={16} /> Browse public collections
        </Link>
        <p className="flex items-center justify-center gap-1 text-xs text-stone-400">
          <Lock size={12} /> Your items stay private unless you make them public.
        </p>
      </div>
    </div>
  )
}
