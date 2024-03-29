import Image from 'next/image'
import dubai from '../../public/dubai.jpg';
import team from '../../public/team.png';

function Contact() {
  return (
    <>
    <div>
      <h3 className="dubai-title" >Dubai, UAE: The QRL Hub</h3>
      <p className="dubai-description" >The QRL Hub is located in Dubai. This is where developers come together, <br></br> from all over the world to collaborate on the next quantum blockchain solutions. <br></br><br></br> Address: [insert address here]</p>
    <Image className="dubai-image" width="500px" height="500px" src={dubai} alt="Dubai" />
    <Image className="team-image" width="500px" height="250px" src={team} alt="QRL Team" />
    </div>
    </>
  );
}

export default Contact;
