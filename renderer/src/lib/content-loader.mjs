import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const renderers = new WeakMap();

// cfgbLoader renders Markdown bodies supplied by the Go metadata index.
// It does not parse front matter; article data is already normalized.
export function cfgbLoader(name, entries) {
  return {
    name,
    load: async (context) => {
      context.store.clear();
      const renderer = await rendererFor(context);
      for (const entry of entries) {
        const data = await context.parseData({
          id: entry.id,
          data: entry.data ?? {},
          filePath: entry.file,
        });
        const fileURL = pathToFileURL(entry.file);
        const result = await renderer.render(entry.body ?? "", {
          frontmatter: {},
          fileURL,
        });
        const rendered = {
          html: result.code,
          metadata: {
            ...result.metadata,
            imagePaths: [
              ...(result.metadata?.localImagePaths ?? []),
              ...(result.metadata?.remoteImagePaths ?? []),
            ],
          },
        };
        context.store.set({
          id: entry.id,
          data,
          body: entry.body,
          filePath: path.relative(fileURLToPath(context.config.root), entry.file),
          digest: context.generateDigest(entry.body ?? ""),
          rendered,
          assetImports: rendered.metadata.imagePaths,
        });
      }
    },
  };
}

async function rendererFor(context) {
  const processor = context.config.markdown?.processor;
  if (!processor?.createRenderer) {
    throw new Error("markdown.processor.createRenderer is required");
  }
  const existing = renderers.get(processor);
  if (existing) return existing;
  const markdown = context.config.markdown;
  const created = processor.createRenderer({
    image: context.config.image,
    syntaxHighlight: markdown.syntaxHighlight,
    shikiConfig: markdown.shikiConfig,
    gfm: markdown.gfm,
    smartypants: markdown.smartypants,
  });
  renderers.set(processor, created);
  return created;
}
