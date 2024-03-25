# Zig and Zoekt hack

fy23-q1-sourcegraph-hackathon

Authors: @stefan and @keegan

Spent a day to see how easy it would be to search a Zoekt shard with zig. We had a very low bar for a goal, which was just enough parsing of a shard file to look for lines that match a needle. Our main goal was to gain some insight into using zig for fun (and profit?).

We succeeded to search the test shard with a hardcoded needle.

Why Zig? Interesting high performance language. Easy integration into other c libraries, so we could potentially use best in class regex engines (eg hyperscan).
