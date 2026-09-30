export default async function Page() {
  const res = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/products`)
  return <pre>{JSON.stringify(await res.json())}</pre>
}
