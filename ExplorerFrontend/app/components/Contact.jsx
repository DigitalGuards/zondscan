import Image from 'next/image'

function Contact() {
  return (
    <>
    <div>
      <Image className="team-image" width="500px" height="250px" src={team} alt="QRL Team" />
    </div>
    </>
  );
}

export default Contact;
