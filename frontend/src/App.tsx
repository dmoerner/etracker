import './App.css'
import AnnounceURL from './AnnounceURL';
import Header from './Header';
import { Link } from 'react-router-dom';
import { useEffect, useState } from "react";

function About() {
  return (
    <>
      <h2>About</h2>

      <p>etracker is an experimental BitTorrent tracker designed to incentivize good peer behavior by rewarding more peers to peers who seed more torrents. In short, the more torrents you seed, and the more data you upload, the more peers you receive. For more information, see <a href="https://github.com/dmoerner/etracker">Github</a>.</p>

      <p>etracker operates with an infohash allowlist. Do not send the operator of this site emails asking for infohashes to be added to the allowlist.</p>
    </>
  )
}



type StatsData = {
  hashcount: number,
  seeders: number,
  leechers: number
}


function Statistics() {



  const [data, setData] = useState<StatsData | undefined>(undefined);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch(window.location.origin + "/api/stats");
        console.log('fetch stats response', response);
        const stats = await response.json();

        setData(stats);
      } catch (error) {
        console.error('Error fetching data:', error);
      }
    };

    fetchData();
  }, []);


  return (
    <>
      <h2>Statistics</h2>
      <ul>
        <li><Link to="/infohashes">Tracked Infohashes: {data && data.hashcount}</Link></li>
        <li>Seeders: {data && data.seeders}</li>
        <li>Leechers: {data && data.leechers}</li>
      </ul>
    </>
  )
}

function App() {
  return (
    <>
      <Header />
      <About />
      <Statistics />
      <AnnounceURL />
    </>
  )
}

export default App
