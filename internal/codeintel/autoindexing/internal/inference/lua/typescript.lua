local json = require "json"
local path = require "path"
local recognizer = require "sg.autoindex.recognizer"
local pattern = require "sg.autoindex.patterns"

local shared = require "sg.autoindex.shared"
local util = require "sg.autoindex.util"

local indexer = require("sg.autoindex.indexes").get "typescript"
local typescript_nmusl_command =
  "N_NODE_MIRROR=https://unofficial-builds.nodejs.org/download/release n --arch x64-musl auto"

local exclude_paths = pattern.new_path_combine(shared.exclude_paths, {
  pattern.new_path_segment "node_modules",
})

local safe_decode = function(encoded)
  local _, payload = pcall(function()
    return json.decode(encoded)
  end)

  return payload
end

local check_lerna_file = function(root, contents_by_path)
  local ancestors = path.ancestors(root)
  for i = 1, #ancestors do
    local payload = safe_decode(contents_by_path[path.join(ancestors[i], "lerna.json")] or "")
    if payload and payload["npmClient"] == "yarn" then
      return true
    end
  end

  return false
end

local infer_typescript_job = function(api, tsconfig_path, should_infer_config)
  local root = path.dirname(tsconfig_path)
  local reverse_ancestors = util.reverse(path.ancestors(tsconfig_path))

  api:register(recognizer.new_path_recognizer {
    patterns = {
      -- To find package roots
      pattern.new_path_basename "package.json",
      -- To disambiguate installation steps
      pattern.new_path_basename "yarn.lock",
      -- Try to determine version
      pattern.new_path_basename ".n-node-version",
      pattern.new_path_basename ".node-version",
      pattern.new_path_basename ".nvmrc",
      -- To reinvoke simple cases with no other files
      pattern.new_path_basename "tsconfig.json",
      pattern.new_path_exclude(exclude_paths),
    },

    patterns_for_content = {
      pattern.new_path_basename "lerna.json",
      pattern.new_path_exclude(exclude_paths),
    },

    -- Invoked when any of the files listed in the patterns above exist.
    generate = function(api, paths, contents_by_path)
      local is_yarn = check_lerna_file(root, contents_by_path)

      local docker_steps = {}
      for i = 1, #reverse_ancestors do
        if util.contains(paths, path.join(reverse_ancestors[i], "package.json")) then
          local install_command = ""
          if is_yarn or util.contains(paths, path.join(reverse_ancestors[i], "yarn.lock")) then
            install_command = "yarn"
          else
            install_command = "npm install"
          end

          local install_command_suffix = ""
          if should_infer_config then
            install_command_suffix = " --ignore-scripts"
          end

          table.insert(docker_steps, {
            root = reverse_ancestors[i],
            image = indexer,
            commands = { install_command .. install_command_suffix },
          })
        end
      end

      local local_steps = {}
      for i = 1, #docker_steps do
        -- Add `n` invocation command before each docker step
        docker_steps[i].commands = util.with_new_head(docker_steps[i].commands, typescript_nmusl_command)
      end

      -- Add `n` invocation (in indexing container) before indexer runs
      local_steps = { typescript_nmusl_command }

      table.insert(
        local_steps,
        'if [ -n "${VM_MEM_MB:-}" ]; then export NODE_OPTIONS="--max-old-space-size=$VM_MEM_MB"; fi'
      )

      local args = { "scip-typescript", "index" }
      if should_infer_config then
        table.insert(args, "--infer-tsconfig")
      end

      return {
        steps = docker_steps,
        local_steps = local_steps,
        root = root,
        indexer = indexer,
        indexer_args = args,
        outfile = "index.scip",
        requested_envvars = { "NPM_TOKEN" },
      }
    end,
  })

  return {}
end

return recognizer.new_path_recognizer {
  patterns = {
    pattern.new_path_basename "package.json",
    pattern.new_path_basename "tsconfig.json",
    pattern.new_path_exclude(exclude_paths),
  },

  -- Invoked when package.json or tsconfig.json files exist
  generate = function(api, paths)
    local has_tsconfig = false
    for i = 1, #paths do
      if path.basename(paths[i]) == "tsconfig.json" then
        -- Infer typescript jobs
        infer_typescript_job(api, paths[i], false)
        has_tsconfig = true
      end
    end

    if not has_tsconfig then
      -- Infer javascript jobs if there's no tsconfig.json found
      infer_typescript_job(api, "tsconfig.json", true)
    end

    return {}
  end,
}
