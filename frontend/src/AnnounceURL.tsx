import { useEffect, useState } from "react";

function keyToURL(key: string): string {
  const url = window.location.href;
  return url + key + '/announce';
}

function AnnounceURL() {

  const [announce, setAnnounce] = useState(localStorage.getItem('announce') || '');
  const announce_url = keyToURL(announce);

  const handleGenerate = () => {
    const fetchData = async () => {
      try {
        const response = await fetch("http://localhost:9000/frontendapi/generate");
        const key = await response.json();

        setAnnounce(key.announce_key)
      } catch (error) {
        console.error('Error fetching data:', error);
      }
    };

    fetchData();
  };

  useEffect(() => {
    localStorage.setItem('announce', announce);
  }, [announce])

  return (
    <>
      <h2>Announce URL</h2>

      <p>Each user of etracker must use their own announce URL to allow the tracker to track statistics across sessions.</p>

      {announce ? (
        <p>Your saved announce URL: <a href={announce_url}>{announce_url}</a></p>
      ) : (
        <p>No announce URL saved</p>
      )}
      <button onClick={handleGenerate}>Generate New Announce URL</button>
    </>
  )
}

export default AnnounceURL;
