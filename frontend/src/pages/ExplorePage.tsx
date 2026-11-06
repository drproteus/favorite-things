import { Compass, ImageOff } from 'lucide-react'
import ItemCard from '../components/ItemCard'
import { EmptyState, ErrorState, Loading } from '../components/Status'
import { useExplore } from '../hooks'

export default function ExplorePage() {
  const explore = useExplore()

  return (
    <div className="space-y-10">
      <div>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Explore</h1>
        <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">Public items shared by collectors, by category.</p>
      </div>

      {explore.isPending && <Loading />}
      {explore.isError && <ErrorState message={explore.error.message} />}
      {explore.data &&
        (explore.data.length === 0 ? (
          <EmptyState icon={<Compass size={40} />} title="Nothing public yet">
            Make some of your items public to see them here.
          </EmptyState>
        ) : (
          explore.data.map((sec) => (
            <section key={sec.category.id} className="space-y-4">
              <div className="flex items-center gap-4">
                <div className="h-20 w-28 shrink-0 overflow-hidden rounded-xl bg-stone-100 dark:bg-stone-800">
                  {sec.cover ? (
                    <img src={sec.cover.url} alt="" loading="lazy" className="h-full w-full object-cover" />
                  ) : (
                    <div className="flex h-full items-center justify-center text-stone-400">
                      <ImageOff size={24} />
                    </div>
                  )}
                </div>
                <div>
                  <h2 className="text-xl font-semibold">{sec.category.name}</h2>
                  <p className="text-sm text-stone-500 dark:text-stone-400">
                    {sec.count} {sec.count === 1 ? 'item' : 'items'}
                  </p>
                </div>
              </div>
              <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                {sec.items.map((item) => (
                  <ItemCard key={item.id} item={item} />
                ))}
              </div>
            </section>
          ))
        ))}
    </div>
  )
}
