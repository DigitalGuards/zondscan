import React from "react";
import Card from 'react-bootstrap/Card';
import Container from 'react-bootstrap/Container';
import Row from 'react-bootstrap/Row';
import ListGroup from 'react-bootstrap/ListGroup';
const axios = require('axios');

function TokensList() {
  const [post, setPost] = React.useState(null);
  React.useEffect(() => {
    axios.get('http://localhost:5000/tokens').then((response) => {
      setPost(response.data.tokens);
    });
  }, []);

  const listItems = post?.map(item =>
    <Row key={item.id}>
      <ListGroup.Item style={{backgroundColor: "#031421", color: "white", textAlign: "center"}}>
        {item.name}
      </ListGroup.Item>
    </Row>
  );
  
  return (
    <>
   <Card style={{width: "75rem", backgroundColor: "#031421"}}>
    <Card.Title style={{color: "white"}}>All Tokens</Card.Title>
    <div class="scrollable">
        <ListGroup>
</ListGroup>
<ListGroup>
<Container>
    {listItems}
  </Container>
</ListGroup>
    </div>
    </Card>
    </>
  );
}

export default TokensList;
