import { Link } from 'react-router-dom';

function Header() {
  return (
    <h1><Link to="/" style={{ textDecoration: "none", color: "black" }}>etracker</Link></h1>
  )
}

export default Header;
