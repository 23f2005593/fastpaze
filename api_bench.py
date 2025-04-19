import asyncio
import time
import httpx

async def benchmark_fastapi(url="http://localhost:8000/hello", num_requests=100, concurrency=10):
    print(f"Benchmarking FastAPI with {num_requests} requests and {concurrency} concurrent users...")
    async with httpx.AsyncClient() as client:
        # Health check
        try:
            response = await client.get(url, timeout=2)
            if response.status_code == 200:
                print("Health check passed for FastAPI")
            else:
                print(f"Health check failed: Status code {response.status_code}")
                return
        except Exception as e:
            print(f"Health check failed: {str(e)}")
            return

        start_time = time.time()
        tasks = []
        successful_requests = 0
        latencies = []

        for i in range(num_requests):
            if i % concurrency == 0 and tasks:
                responses = await asyncio.gather(*tasks, return_exceptions=True)
                for resp in responses:
                    if not isinstance(resp, Exception):
                        successful_requests += 1
                        if hasattr(resp, 'elapsed'):
                            latencies.append(resp.elapsed.total_seconds() * 1000)  # Convert to ms
                tasks = []
            tasks.append(client.get(url))

        if tasks:
            responses = await asyncio.gather(*tasks, return_exceptions=True)
            for resp in responses:
                if not isinstance(resp, Exception):
                    successful_requests += 1
                    if hasattr(resp, 'elapsed'):
                        latencies.append(resp.elapsed.total_seconds() * 1000)

        end_time = time.time()
        total_time = end_time - start_time
        rps = successful_requests / total_time if total_time > 0 else 0
        avg_latency = sum(latencies) / len(latencies) if latencies else 0
        min_latency = min(latencies) if latencies else 0
        max_latency = max(latencies) if latencies else 0

        print(f"Results for FastAPI:")
        print(f"  Total Requests: {successful_requests} (out of {num_requests} attempted)")
        print(f"  Total Time: {total_time:.2f} seconds")
        print(f"  Requests per Second (RPS): {rps:.2f}")
        print(f"  Average Latency: {avg_latency:.2f} ms")
        print(f"  Min Latency: {min_latency:.2f} ms")
        print(f"  Max Latency: {max_latency:.2f} ms")
        print("----------------------------------------")

if __name__ == "__main__":
    for concurrency in [10, 50]:
        asyncio.run(benchmark_fastapi(num_requests=500, concurrency=concurrency))