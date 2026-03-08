vim.lsp.enable('lux')

vim.diagnostic.config({
	virtual_text = false,
})

local opts = { noremap = true, silent = true }

vim.keymap.set("n", "gd", vim.lsp.buf.definition, opts)
vim.keymap.set("n", "K", vim.lsp.buf.hover, opts)
vim.keymap.set("n", "gr", vim.lsp.buf.references, opts)
vim.keymap.set("n", "<leader>r", vim.lsp.buf.rename, opts)
vim.keymap.set("n", "<leader>a", vim.lsp.buf.code_action, opts)
vim.keymap.set("n", "[d", vim.diagnostic.goto_prev, opts)
vim.keymap.set("n", "]d", vim.diagnostic.goto_next, opts)
vim.keymap.set("n", "<leader>d", vim.diagnostic.open_float, opts)
