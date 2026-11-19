export async function onRequestGet({ env }) {
  const result = await env.DB.prepare('SELECT 1 AS ready').first();
  return Response.json({ ok: result?.ready === 1 });
}
