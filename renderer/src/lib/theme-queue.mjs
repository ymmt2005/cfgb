// Serialize theme redraws. A newer request supersedes one that is already
// running: the in-flight draw must not publish after it loses the token, and
// only the latest request performs the next draw.
export function createThemeQueue(draw) {
  let tail = Promise.resolve();
  let ticket = 0;
  return function request() {
    const mine = ++ticket;
    const current = () => mine === ticket;
    tail = tail.then(() => draw(current)).catch(() => {});
    return tail;
  };
}
