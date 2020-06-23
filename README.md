# serialConnector

## Todos

* reader must provide characters. These string must be processed by lexer
* statemachine just takes TOKENS and changes state according tokens.

* Each byte received is read and appended to a token string
* As soon as the token string matches a regex, the whole token string is 
  removed and the corresponding token is the next that nextToken() returns
* Each string sent to the channel must be corresponding to a state in the 
  state machine.

```
undefined--recv("login:")--
-->loggedoff ---send(username)--> usernamesent --recv("Password:")--
-->awaitingPassword --send(password)--> passwordsent --
--recv("Login failed") --> loginfailed
--recv("Last login") --> loggedin
```

## Later

* yaml file to configure host or target, commands, init,test,result scripts etc.
