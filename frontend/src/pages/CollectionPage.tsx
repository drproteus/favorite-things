import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import ItemBrowser from '../components/ItemBrowser'
import { btnPrimary } from '../ui'

export default function CollectionPage() {
  return (
    <ItemBrowser
      scope="mine"
      title="My collection"
      subtitle="Everything you've cataloged, public or private."
      actions={
        <Link to="/items/new" className={btnPrimary}>
          <Plus size={16} /> New item
        </Link>
      }
    />
  )
}
