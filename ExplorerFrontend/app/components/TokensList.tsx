import { useState, useEffect } from "react";
import Card from 'react-bootstrap/Card';
import Container from 'react-bootstrap/Container';
import Row from 'react-bootstrap/Row';
import ListGroup from 'react-bootstrap/ListGroup';
import axios from 'axios';

interface Token {
  id: string;
  name: string;
}

export default function TokensList(): JSX.Element {
  const [tokens, setTokens] = useState<Token[] | null>(null);

  useEffect(() => {
    const fetchTokens = async (): Promise<void> => {
      try {
        const response = await axios.get('http://localhost:5000/tokens');
        setTokens(response.data.tokens);
      } catch (error) {
        console.error('Error fetching tokens:', error);
      }
    };

    fetchTokens();
  }, []);

  const listItems = tokens?.map(item => (
    <Row key={item.id}>
      <ListGroup.Item style={{backgroundColor: "#031421", color: "white", textAlign: "center"}}>
        {item.name}
      </ListGroup.Item>
    </Row>
  ));
  
  return (
    <Card style={{width: "75rem", backgroundColor: "#031421"}}>
      <Card.Title style={{color: "white"}}>All Tokens</Card.Title>
      <div className="scrollable">
        <ListGroup>
          <Container>
            {listItems}
          </Container>
        </ListGroup>
      </div>
    </Card>
  );
}
