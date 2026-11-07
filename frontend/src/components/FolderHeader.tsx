import type { Collection } from '../types'
import Markdown from './Markdown'
import ResponsiveImage from './ResponsiveImage'

// The banner and description shown at the top of a folder. Renders nothing when a folder has neither.
export default function FolderHeader({ folder }: { folder: Collection }) {
  if (!folder.bannerUrl && !folder.description) return null
  return (
    <div className="space-y-4">
      {folder.bannerUrl && (
        <ResponsiveImage
          image={{
            url: folder.bannerUrl,
            width: folder.bannerWidth,
            height: folder.bannerHeight,
            variants: folder.bannerVariants,
          }}
          sizes="(min-width: 1024px) 880px, 100vw"
          alt=""
          className="h-40 w-full rounded-2xl object-cover sm:h-52"
          loading="lazy"
        />
      )}
      {folder.description && <Markdown text={folder.description} className="text-stone-700 dark:text-stone-300" />}
    </div>
  )
}
