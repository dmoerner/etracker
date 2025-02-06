import './App.css'

function About() {
  return (
    <>
      <h2>About</h2>

      <p>etracker is an experimental BitTorrent tracker designed to incentivize good peer behavior by rewarding more peers to peers who seed more torrents. In short, the more torrents you seed, and the more data you upload, the more peers you receive. For more information, see <a href="https://github.com/dmoerner/etracker">Github</a>.</p>

      <p>etracker operates with an infohash allowlist. Do not send the operator of this site emails asking for infohashes to be added to the allowlist.</p>
    </>
  )
}

async function Statistics() {
  const response = await fetch("/frontend/stats");
  // TODO: Errors
  const stats = await response.json();
  return (
    <>
      <h2>Statistics</h2>
      <ul>
        <li>Tracked Infohashes: </li>
        <li>Tracked Infohashes: </li>
        <li>Tracked Infohashes: </li>
      </ul>
    </>
  )
}

async function AnnounceURL() {
  return (
    <>
      <h2>Announce URL</h2>

      <p>etracker is like a hybrid public/private tracker. Although registration is not required, each user must use a unique announce URL. This allows the tracker to track statistics across sessions and reward good seeders with better peer lists.</p>
    </>
  )
}

function App() {
  return (
    <>
      <h1>etracker</h1>
      <About />
      <Statistics />
      <AnnounceURL />
    </>
  )
}

export default App
