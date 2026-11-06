import { useState } from 'react'
import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { LogOut, Menu, Sparkles, X } from 'lucide-react'
import { useLogout, useMe } from '../hooks'
import { btnPrimary, btnGhost } from '../ui'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `text-sm font-medium transition ${
    isActive ? 'text-indigo-600 dark:text-indigo-400' : 'text-stone-600 hover:text-stone-900 dark:text-stone-400 dark:hover:text-stone-100'
  }`

export default function Layout() {
  const { data: me } = useMe()
  const logout = useLogout()
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)

  const links = [
    ...(me ? [{ to: '/collection', label: 'My collection' }] : []),
    ...(me ? [{ to: '/settings', label: 'Settings' }] : []),
    { to: '/explore', label: 'Explore' },
  ]

  const signOut = () => {
    logout.mutate(undefined, { onSettled: () => navigate('/login') })
    setMenuOpen(false)
  }

  return (
    <div className="min-h-dvh bg-stone-50 text-stone-900 dark:bg-stone-950 dark:text-stone-100">
      <header className="sticky top-0 z-30 border-b border-stone-200 bg-white/80 backdrop-blur dark:border-stone-800 dark:bg-stone-900/80">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4">
          <div className="flex items-center gap-8">
            <Link to="/" className="flex items-center gap-2 font-semibold tracking-tight">
              <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-600 text-white">
                <Sparkles size={16} />
              </span>
              Favorite Things
            </Link>
            <nav className="hidden items-center gap-6 md:flex">
              {links.map((l) => (
                <NavLink key={l.to} to={l.to} className={navLinkClass}>
                  {l.label}
                </NavLink>
              ))}
            </nav>
          </div>

          <div className="flex items-center gap-2">
            {me ? (
              <>
                <div className="hidden items-center gap-2 md:flex">
                  {me.avatarUrl ? (
                    <img src={me.avatarUrl} alt="" className="h-8 w-8 rounded-full" referrerPolicy="no-referrer" />
                  ) : (
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-stone-200 text-sm font-semibold dark:bg-stone-700">
                      {me.name.charAt(0).toUpperCase()}
                    </span>
                  )}
                  <button onClick={signOut} className={btnGhost} aria-label="Sign out">
                    <LogOut size={16} />
                  </button>
                </div>
                <button
                  className={`${btnGhost} md:hidden`}
                  onClick={() => setMenuOpen((o) => !o)}
                  aria-label={menuOpen ? 'Close menu' : 'Open menu'}
                  aria-expanded={menuOpen}
                >
                  {menuOpen ? <X size={20} /> : <Menu size={20} />}
                </button>
              </>
            ) : (
              <Link to="/login" className={btnPrimary}>
                Sign in
              </Link>
            )}
          </div>
        </div>

        {menuOpen && me && (
          <nav className="border-t border-stone-200 px-4 py-3 md:hidden dark:border-stone-800">
            <div className="flex flex-col gap-1">
              {links.map((l) => (
                <NavLink
                  key={l.to}
                  to={l.to}
                  onClick={() => setMenuOpen(false)}
                  className={({ isActive }) =>
                    `rounded-lg px-3 py-2 ${isActive ? 'bg-indigo-50 dark:bg-indigo-950' : 'hover:bg-stone-100 dark:hover:bg-stone-800'} ${navLinkClass({ isActive })}`
                  }
                >
                  {l.label}
                </NavLink>
              ))}
              <button onClick={signOut} className="rounded-lg px-3 py-2 text-left text-sm font-medium text-stone-600 hover:bg-stone-100 dark:text-stone-400 dark:hover:bg-stone-800">
                Sign out{me.name ? ` (${me.name})` : ''}
              </button>
            </div>
          </nav>
        )}
      </header>

      <main className="mx-auto max-w-6xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  )
}
