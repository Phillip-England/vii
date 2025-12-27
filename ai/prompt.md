---
$at("*")
$skill("ergo")
$skill("modular")
$skill("rpp")
---


okay we need to make vii a cli tool that will allow us to do the following:

vii init // creates a default main.go with LoggerService included also installs vii using go install as well the latest version and this should not work is a main.go exists unless we do --force with it
vii route some_route.go // creates default route called SomeRoute
vii service some_service.go // creates default service called SomeService
vii validator some_validator.go // creates default validator called SomeValidator

the vii route, vii service, and vii validator must always resolve the name of the new type as PasalCase even if the user puts in any format.

It should account for camelCase snake_case and hyphen-case

then itll be converted into PascalCase for the struct name please

this should allow users to quickly create new services, routes,and validatos with ease.

please figure out the best way to organize our code and plae this in main.go