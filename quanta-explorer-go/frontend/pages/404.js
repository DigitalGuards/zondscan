import Image from 'next/image'

function NoPage() {

  var images = new Array("/titanic.png", "/squirrel01.png", "/squirrel02.png", "/squirrel03.png", "/squirrel04.png", "/squirrel05.png", "/squirrel06.png");
  var randomImage = images[Math.floor(Math.random() * images.length)];

  return (
    <div className="no-page">
      <Image className="no-page-image" width={"250"} height={"250"} src={randomImage} alt="404 image" />
      <h3>404 Error: Page Not Found :(</h3>
    </div>
  );
}

export default NoPage;
