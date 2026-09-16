# TUTORIAL TO INSTALL WASA PROJECT VALIDATOR:
*(https://gist.github.com/ENDERZOMBI102/7a53044dfc86fc82e9f16923d950c000)*

1. Install **brew**
2. run: brew install **node**

---

## YAML VALIDATOR:
3. copy **.spectral.js** in your project root directory
4. add .spectral.js in .git/info/exclude
5. run: npm install -g ibm-openapi-validator@1.16.3
6. to verify the yaml code run: **lint-openapi doc/api.yaml**

---

## GO VALIDATOR:
6. copy **.golangci.yml** in your project root directory		
7. install standard Go linter: brew install golangci-lint
8. to verify the backend code run: **golangci-lint run**

---

## VUE VALIDATOR:
9. copy **eslint.config.mjs** in your project webui directory
10. install eslint and eslint-plugin-vue: cd webui/ npm install -D eslint@9.2.0 eslint-plugin-vue@9.25.0
11. add "lint": "node node_modules/eslint/bin/eslint.js ." to webui/package.json like this:
```
"scripts": {
  ...
  "lint": "node node_modules/eslint/bin/eslint.js ."
}
```
12. to verify the frontend code run: **npm run lint**