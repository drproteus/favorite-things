import ItemBrowser from '../components/ItemBrowser'
import TopTags from '../components/TopTags'

export default function ExplorePage() {
  return (
    <ItemBrowser
      scope="public"
      title="Explore"
      subtitle="Public items shared by collectors."
      before={<TopTags />}
    />
  )
}
