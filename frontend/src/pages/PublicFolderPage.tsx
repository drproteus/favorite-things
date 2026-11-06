import { Link, useParams } from 'react-router-dom'
import { Folder } from 'lucide-react'
import FolderHeader from '../components/FolderHeader'
import ItemBrowser from '../components/ItemBrowser'
import { EmptyState, ErrorState, Loading } from '../components/Status'
import { childrenOf } from '../folders'
import { usePublicCollections } from '../hooks'
import { card } from '../ui'

// A public folder on its own: its banner, description, subfolders and items. Nothing above it is shown.
export default function PublicFolderPage() {
  const { id = '', folderId = '' } = useParams()
  const folders = usePublicCollections(id)

  if (folders.isPending) return <Loading />
  if (folders.error) return <ErrorState message={folders.error.message} />

  const folder = folders.data.find((c) => c.id === folderId)
  if (!folder) {
    return (
      <EmptyState icon={<Folder size={40} />} title="Folder not found">
        This folder isn't shared publicly.
      </EmptyState>
    )
  }
  const subfolders = childrenOf(folders.data, folder.id)

  return (
    <ItemBrowser
      scope="public"
      owner={id}
      title={folder.name}
      collectionId={folder.id}
      before={<FolderHeader folder={folder} />}
      folders={() =>
        subfolders.length > 0 && (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {subfolders.map((f) => (
              <Link
                key={f.id}
                to={`/users/${id}/folders/${f.id}`}
                className={`${card} flex items-center gap-3 p-4 transition hover:-translate-y-0.5 hover:shadow-md`}
              >
                <Folder size={20} className="shrink-0 text-indigo-500" />
                <span className="block truncate font-medium">{f.name}</span>
              </Link>
            ))}
          </div>
        )
      }
    />
  )
}
