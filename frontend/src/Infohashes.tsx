import Header from "./Header";
import { useState, useEffect } from "react";

type InfohashesData = {
  name: string,
  info_hash: string,
  downloads: number,
  seeders: number,
  leechers: number,
}

// The infohash is marshalled into b64 JSON, but the GET endpoint expects hex.
function b64ToHex(b64: string): string {
  const bin = atob(b64);
  let hex = '';
  for (let i = 0; i < bin.length; i++) {
    const byte = bin.charCodeAt(i).toString(16);
    hex += byte.padStart(2, '0');
  }
  return hex;
}

function DownloadTorrent({ infohash, name, announce }: { infohash: string, name: string, announce: string }) {
  infohash = b64ToHex(infohash);

  const handleClick = async (infohash: string) => {
    const fetchTorrent = async () => {
      try {
        const response = await fetch(window.location.origin + `/api/torrentfile?announce_key=${announce}&info_hash=${infohash}`)

        if (!response.ok) {
          const error = await response.json();
          throw new Error(error.message);
        }

        const blob = await response.blob();

        return blob;
      } catch (error) {
        console.error('Error fetching torrent file:', error);
      }
    }

    try {
      const blob = await fetchTorrent();
      if (blob === undefined) {
        throw new Error("No blob received from API")
      }
      const url = window.URL.createObjectURL(blob);

      const link = document.createElement('a');
      link.href = url;
      link.download = `${name}.torrent`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Error downloading torrent file:', error);
    }
  }

  return (<button onClick={() => handleClick(infohash)}>Download</button>)

}

function Table({ data }: { data: InfohashesData[] }) {
  const [announce, _] = useState(localStorage.getItem('announce') || '');

  return (
    <table>
      <thead>
        <tr>
          {announce && <th key="download">download</th>}
          {data.length > 0 && Object.keys(data[0]).map(key => (
            <th key={key}>{key}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.length > 0 && data.map((row, index) => (
          <tr key={index}>
            <td key={`${index}_button`}><DownloadTorrent infohash={row.info_hash} name={row.name} announce={announce} /></td>
            {Object.values(row).map((value, index2) => (
              <td key={index2}>{value}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table >
  )
}

function Infohashes() {
  const [data, setData] = useState<InfohashesData[] | undefined>(undefined);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch(window.location.origin + "/api/infohashes");
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
      <Header />

      <h2>Tracked Infohashes</h2>
      {data && <Table data={data} />}
    </>
  )
}

export default Infohashes;
