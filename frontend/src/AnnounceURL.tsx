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
        const response = await fetch(window.location.origin + "/api/generate");
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

      <p>Each user of etracker must use their own announce URL to allow the tracker to track statistics across sessions. To accurately report stats, do not merge this announce URL with other announce URLs in the same torrent in your client. Custom announce URLs generated below are pruned 3 months after creation or 3 months after the last announce, whichever is longer.</p>

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
