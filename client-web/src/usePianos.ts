import { useQuery } from "@tanstack/react-query";
import { fetchState, type Piano } from "./api";

const NO_PIANOS: Piano[] = [];

// Polls the server for all pianos. `now` is the server's clock at that poll, so "x min ago"
// doesn't depend on the client clock and everything that shows it refreshes with each poll.
export function usePianos() {
    const { data, error } = useQuery({
        queryKey: ["state"],
        queryFn: ({ signal }) => fetchState(signal),
        refetchInterval: 1000, // new segments show up within a second, part of the delay behind the piano
        refetchIntervalInBackground: true, // keep the tuned station fed while the tab is hidden
        retry: false, // the next poll is the retry
    });
    return {
        pianos: data?.pianos ?? NO_PIANOS,
        now: data?.now ?? "",
        error: error?.message ?? null,
    };
}
