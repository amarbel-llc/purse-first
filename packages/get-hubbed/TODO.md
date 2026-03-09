
- [ ] ⏺ plugin:get-hubbed:get-hubbed - content_read (MCP)(repo: "NotWoods/rollup-plugin-consts", path: "src/index.ts")         
  ⎿  Error: gh api contents: gh [api repos/NotWoods/rollup-plugin-consts/contents/src/index.ts --method GET]: exit status
      1: gh: Not Found (HTTP 404)        
- [ ] ⏺ plugin:get-hubbed:get-hubbed - content_read (MCP)(repo: "RustCrypto/SSH", path: "ssh-key/src/public/key_data.rs", ref:
                                                   "ssh-key-v0.6.0", line_offset: 270, line_limit: 50)
  ⎿  Error: gh api contents: gh [api repos/RustCrypto/SSH/contents/ssh-key/src/public/key_data.rs --method GET -f        
     ref=ssh-key-v0.6.0]: exit status 1: gh: No commit found for the ref ssh-key-v0.6.0 (HTTP 404)
- [ ] ● Bash(gh pr view --json title,body,url,number 2>/dev/null || echo "NO_PR_FOUND")
