// SPDX-License-Identifier: MIT
pragma solidity ^0.8.13;

contract Vote {
    string public title;
    string public originator;
    uint public blockheight;
    string public eligibility;
    string public excluded;
    string public mechanics;
    string public expires;
    string[] public options;
    mapping (string => uint) public votes;
    mapping (address => bool) public hasVoted;
    address private admin;

    constructor(
        string memory _title, 
        string memory _originator, 
        uint _blockheight, 
        string memory _eligibility, 
        string memory _excluded, 
        string memory _mechanics, 
        string memory _expires, 
        string memory adminPass
    ) {
        title = _title;
        originator = _originator;
        blockheight = _blockheight;
        eligibility = _eligibility;
        excluded = _excluded;
        mechanics = _mechanics;
        expires = _expires;
        options.push("DARK MODE");
        options.push("LIGHT MODE");
        require(keccak256(abi.encodePacked(adminPass)) == keccak256(abi.encodePacked("CHANGEME")), "Invalid admin password");
        admin = msg.sender;
    }

    function vote(string memory option) public {
        // here you need to implement the logic to check the balance of msg.sender
        require(hasVoted[msg.sender] == false, "User has already voted");
        require(isOptionValid(option), "Invalid voting option");
        hasVoted[msg.sender] = true;
        votes[option]++;
    }

    function getVotes(string memory option) public view returns (uint) {
        return votes[option];
    }

    function isOptionValid(string memory option) private view returns (bool) {
        for(uint i = 0; i < options.length; i++) {
            if(keccak256(abi.encodePacked(options[i])) == keccak256(abi.encodePacked(option))) {
                return true;
            }
        }
        return false;
    }
}