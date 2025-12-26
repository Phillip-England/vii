---
$at("*")
$skill("rpp")
$skill("ergo")
$skill("modular")
---

Okay we are close, but I want each websocket route to be its own indepent agent apart from the other methods associated with it. Just because a MESSAGE and CLOSE exist on the same endpoint does not mean they exist on the same handler.

No, each type has its own Handler. Then, I should just be able to access the message, code, reason, ect from within the endpoint using the request context if possible with helper functions? IDK but I do not want them grouped, that way they each haave their own middleware and validators.