"""Regression checks for CI's partition and aggregate correctness gates."""
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("shards", Path(__file__).with_name("test-shards.py"))
shards = importlib.util.module_from_spec(spec)
spec.loader.exec_module(shards)


class ShardTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def test_partition(self):
        listing = self.root / "list.txt"
        listing.write_text("TestZ\nBenchmarkIgnore\nExampleA\nFuzzB\nTestA\nok package\n")
        patterns = []
        for i in (1, 2):
            out = self.root / str(i)
            shards.split(listing, out, i)
            patterns.append((out / "tsfront-run.txt").read_text().strip())
        self.assertEqual(patterns, ["^(ExampleA|TestA)$", "^(FuzzB|TestZ)$"])
        listing.write_text("TestOne\n")
        shards.split(listing, self.root / "empty", 2)
        self.assertEqual((self.root / "empty/tsfront-run.txt").read_text(), "^$\n")
        listing.write_text("TestB\nTestA\nTestC\n")
        shards.split(listing, self.root / "ts", 2, "tsfront")
        self.assertEqual((self.root / "ts/tsfront-run.txt").read_text(), "^(TestB)$\n")
        self.assertEqual(json.loads((self.root / "ts/tsfront-listed.json").read_text()), ["TestA", "TestB", "TestC"])

    def fixtures(self):
        # Shard 1 runs another package and tsfront's first half; shard 2
        # runs tsfront's second half.
        tests = {1: [(shards.TSFRONT, "TestA"), ("github.com/oisee/abapiti/hir", "Test3")],
                 2: [(shards.TSFRONT, "TestB")]}
        for i, rows in tests.items():
            out = self.root / f"test-shard-{i}"
            out.mkdir()
            events = []
            for pkg, name in rows:
                events += [dict(Package=pkg, Action="run", Test=name),
                           dict(Package=pkg, Action="pass", Test=name),
                           dict(Package=pkg, Action="pass", Elapsed=i)]
            (out / "test.json").write_text("\n".join(json.dumps(e) for e in events) + "\n")
            (out / "tsfront-listed.json").write_text('["TestA", "TestB"]')
            pkg = rows[0][0]
            (out / "cover.out").write_text(
                f"mode: atomic\n{pkg}/x.go:1.1,2.2 2 {int(i != 1)}\n"
                f"{pkg}/x.go:3.1,4.2 2 0\n")

    def test_aggregate(self):
        self.fixtures()
        shards.check(self.root)
        (self.root / "test-shard-2/tree-dirty").touch()
        out = self.root / "merged"
        shards.aggregate(self.root, out)
        self.assertTrue((out / "tree-dirty").exists())
        rows = {p['pkg']: p for p in json.loads((out / 'test-summary.json').read_text())['packages']}
        self.assertEqual(rows[shards.TSFRONT]['seconds'], 3)
        self.assertEqual(rows[shards.TSFRONT]['pass'], 2)
        self.assertEqual(rows['github.com/oisee/abapiti/hir']['pass'], 1)
        self.assertEqual(rows[shards.TSFRONT]['cover'], 50)
        self.assertEqual(len((out / 'cover.out').read_text().splitlines()), 3)

    def test_missing_duplicate_and_different_lists(self):
        self.fixtures()
        log = self.root / "test-shard-1/test.json"
        original = log.read_text()
        log.write_text(original + original.splitlines()[0] + '\n')
        with self.assertRaisesRegex(ValueError, 'extra/duplicate'):
            shards.check(self.root)
        log.write_text('\n'.join(original.splitlines()[1:]) + '\n')
        with self.assertRaisesRegex(ValueError, 'missing'):
            shards.check(self.root)
        log.write_text(original)
        (self.root / 'test-shard-2/tsfront-listed.json').write_text('["TestB"]')
        with self.assertRaisesRegex(ValueError, 'lists differ'):
            shards.check(self.root)
        (self.root / 'test-shard-2/tsfront-listed.json').write_text('["TestA", "TestB"]')
        (self.root / 'test-shard-2/test.json').unlink()
        with self.assertRaises(FileNotFoundError):
            shards.check(self.root)

    def test_merge(self):
        a, b, out = [self.root / n for n in ('a', 'b', 'out')]
        a.write_text('mode: atomic\nx.go:1.1,2.2 2 5\nx.go:3.1,4.2 3 0\n')
        b.write_text('mode: atomic\nx.go:1.1,2.2 2 2\nx.go:3.1,4.2 3 4\ny.go:1.1,2.2 1 7\n')
        shards.merge([a, b], out)
        self.assertEqual(out.read_text(), 'mode: atomic\nx.go:1.1,2.2 2 5\nx.go:3.1,4.2 3 4\ny.go:1.1,2.2 1 7\n')
        b.write_text('mode: count\n')
        with self.assertRaisesRegex(ValueError, 'modes differ'):
            shards.merge([a, b], out)
        b.write_text('mode: atomic\nx.go:1.1,2.2 7 1\n')
        with self.assertRaisesRegex(ValueError, 'inconsistent block'):
            shards.merge([a, b], out)


if __name__ == '__main__':
    unittest.main()
