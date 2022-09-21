local fun = require "fun"
local path = require "path"
local pattern = require "sg.autoindex.patterns"
local recognizer = require "sg.autoindex.recognizer"

local indexer = "sourcegraph/scip-java"
local outfile = "index.scip"

local is_project_structure_supported = function(base)
  return base == "pom.xml" or base == "build.gradle" or base == "build.gradle.kts"
end

return recognizer.new_path_recognizer {
  patterns = {
    pattern.new_path_extension_set { "java", "scala", "kt" },
    pattern.new_path_basename_set { "pom.xml", "build.gradle", "build.gradle.kts" },
  },

  -- Invoked when Java, Scala, Kotlin, or Gradle build files exist
  generate = function(api)
    api:register(recognizer.new_path_recognizer {
      patterns = {
        pattern.new_path_literal "lsif-java.json",
      },

      -- Invoked when lsif-java.json exists in root of repository
      generate = function(_, _)
        return {
          steps = {},
          root = "",
          indexer = indexer,
          indexer_args = { "scip-java", "index", "--build-tool=scip" },
          outfile = outfile,
        }
      end,
    })

    return {}
  end,

  -- Invoked when Java, Scala, Kotlin, or Gradle build files exist
  hints = function(_, paths)
    local hints = {}
    local visited = {}

    fun.each(function(p)
      local dir, base = path.split(p)

      if visited[dir] == nil and is_project_structure_supported(base) then
        table.insert(hints, {
          root = dir,
          indexer = indexer,
          confidence = "PROJECT_STRUCTURE_SUPPORTED",
        })

        visited[dir] = true
      end
    end, paths)

    fun.each(function(p)
      local dir, base = path.split(p)

      if visited[dir] == nil and not is_project_structure_supported(base) then
        table.insert(hints, {
          root = dir,
          indexer = indexer,
          confidence = "LANGUAGE_SUPPORTED",
        })

        visited[dir] = true
      end
    end, paths)

    return hints
  end,
}
