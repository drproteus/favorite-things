import type { ImgHTMLAttributes } from 'react'
import type { ImageSource } from '../types'

interface Props extends Omit<ImgHTMLAttributes<HTMLImageElement>, 'src' | 'srcSet' | 'sizes'> {
  image?: ImageSource | null
  // The rendered width at each breakpoint, so the browser can pick the smallest
  // variant that still looks sharp. See the `sizes` attribute.
  sizes: string
}

// Renders the original image with a srcset of generated sizes. Browsers choose the
// most efficient variant for the element's width and pixel density. Images uploaded
// before variants existed simply have no srcset and keep working.
export default function ResponsiveImage({ image, sizes, ...props }: Props) {
  if (!image?.url) return null

  const variants = [...(image.variants ?? [])].sort((a, b) => a.width - b.width)
  const srcSet = variants.length > 0 ? variants.map((v) => `${v.url} ${v.width}w`).join(', ') : undefined
  // Browsers that support srcset ignore src, so use the largest version as the fallback.
  const src = variants.length > 0 ? variants[variants.length - 1].url : image.url

  return (
    <img
      {...props}
      src={src}
      srcSet={srcSet}
      sizes={srcSet ? sizes : undefined}
      width={image.width || undefined}
      height={image.height || undefined}
      decoding="async"
    />
  )
}
