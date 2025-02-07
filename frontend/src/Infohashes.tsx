import Header from "./Header";
import { useState, useEffect } from "react";

type InfohashesData = {
  Name: string,
  Infohash: string,
  Downloads: number,
  Seeders: number,
  Leechers: number,
}

function Table({ data }: { data: InfohashesData[] }) {
  return (
    <table>
      <thead>
        <tr>
          {Object.keys(data[0]).map(key => (
            <th key={key}>{key}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map((row, index) => (
          <tr key={index}>
            {Object.values(row).map((value, index2) => (
              <td key={index2}>{value}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function Infohashes() {
  const [data, setData] = useState<InfohashesData[] | undefined>(undefined);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch("http://localhost:9000/frontendapi/infohashes");
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
