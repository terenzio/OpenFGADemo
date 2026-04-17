// Package main implements a scripted, narrated CLI demo that walks through
// 8 chapters teaching Relationship-Based Access Control (ReBAC) concepts
// using OpenFGA.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	openfga "github.com/openfga/go-sdk"
	"github.com/terenzio/OpenFGADemo/internal/fga"
)

var (
	pauseFlag bool
	client    *fga.Client
	chapter   int
)

func main() {
	flag.BoolVar(&pauseFlag, "pause", false, "pause between chapters (press Enter to continue)")
	flag.Parse()

	apiURL := envOrDefault("FGA_API_URL", "http://localhost:8080")
	modelPath := envOrDefault("FGA_MODEL_PATH", "model/authorization-model.fga")

	ctx := context.Background()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║         OpenFGA ReBAC Teaching Walkthrough                      ║")
	fmt.Println("║         Learn Relationship-Based Access Control in 8 chapters   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Chapter 1: Bootstrap
	chapter1(ctx, apiURL, modelPath)
	waitForEnter()

	// Chapter 2: Direct Permissions
	chapter2(ctx)
	waitForEnter()

	// Chapter 3: Folder Inheritance
	chapter3(ctx)
	waitForEnter()

	// Chapter 4: Nested Folders
	chapter4(ctx)
	waitForEnter()

	// Chapter 5: Groups via Usersets
	chapter5(ctx)
	waitForEnter()

	// Chapter 6: Public Sharing (Wildcards)
	chapter6(ctx)
	waitForEnter()

	// Chapter 7: Reverse Queries (ListObjects)
	chapter7(ctx)
	waitForEnter()

	// Chapter 8: Expand (Resolution Tree)
	chapter8(ctx)

	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════")
	fmt.Println("  Demo Complete!")
	fmt.Println()
	fmt.Println("  Concepts covered:")
	fmt.Println("    1. Store & model bootstrap")
	fmt.Println("    2. Direct permissions (owner/editor/viewer)")
	fmt.Println("    3. Folder inheritance (parent relationship)")
	fmt.Println("    4. Nested folders (multi-hop inheritance)")
	fmt.Println("    5. Groups via usersets (organization members)")
	fmt.Println("    6. Public sharing (wildcard access)")
	fmt.Println("    7. Reverse queries (ListObjects)")
	fmt.Println("    8. Expand (resolution trees)")
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    - Try the HTTP server: go run ./cmd/server")
	fmt.Println("    - Explore the Playground: http://localhost:3000")
	fmt.Println("══════════════════════════════════════════════════════════════════")
	fmt.Println()
}

// ---------------------------------------------------------------------------
// Chapter implementations
// ---------------------------------------------------------------------------

func chapter1(ctx context.Context, apiURL, modelPath string) {
	printHeader("Bootstrap")

	fmt.Println("  Creating a new OpenFGA store and writing the authorization model...")
	fmt.Println()

	var modelID string
	var err error
	client, modelID, err = fga.EnsureStoreAndModel(ctx, apiURL, "openfga-demo-cli", modelPath)
	if err != nil {
		log.Fatalf("  Failed to bootstrap: %v\n", err)
	}

	fmt.Printf("  Store created and model written successfully.\n")
	fmt.Printf("  Model ID: %s\n", modelID)
	fmt.Println()
	fmt.Println("  Tip: Open the OpenFGA Playground at http://localhost:3000 to visualize")
	fmt.Println("  the model and relationships as we build them.")
}

func chapter2(ctx context.Context) {
	printHeader("Direct Permissions")

	fmt.Println("  Concept: Users can be directly assigned roles on objects.")
	fmt.Println("  Model rule: owner implies editor, editor implies viewer.")
	fmt.Println("  So an owner can automatically view and edit.")
	fmt.Println()

	writeTuples(ctx, "Making alice the owner of document:roadmap",
		fga.Tuple("user:alice", "owner", "document:roadmap"),
	)

	check(ctx, "Can alice view document:roadmap?",
		"user:alice", "viewer", "document:roadmap",
		"alice is owner, owner implies editor, editor implies viewer",
	)

	check(ctx, "Can bob view document:roadmap?",
		"user:bob", "viewer", "document:roadmap",
		"bob has no relationship to document:roadmap",
	)
}

func chapter3(ctx context.Context) {
	printHeader("Folder Inheritance")

	fmt.Println("  Concept: Documents inherit permissions from their parent folder.")
	fmt.Println("  Model rule: 'editor from parent' means if you're editor on the folder,")
	fmt.Println("  you're editor on any document inside it.")
	fmt.Println()

	writeTuples(ctx, "Placing document:roadmap under folder:product, making charlie editor of folder",
		fga.Tuple("folder:product", "parent", "document:roadmap"),
		fga.Tuple("user:charlie", "editor", "folder:product"),
	)

	check(ctx, "Can charlie edit document:roadmap?",
		"user:charlie", "editor", "document:roadmap",
		"charlie is editor on folder:product, document:roadmap's parent is folder:product, 'editor from parent' grants it",
	)

	check(ctx, "Can charlie view document:roadmap?",
		"user:charlie", "viewer", "document:roadmap",
		"editor implies viewer, so charlie can also view",
	)
}

func chapter4(ctx context.Context) {
	printHeader("Nested Folders")

	fmt.Println("  Concept: Permissions flow through nested folder hierarchies.")
	fmt.Println("  If folder:company is parent of folder:product, and folder:product is")
	fmt.Println("  parent of document:roadmap, permissions cascade through the chain.")
	fmt.Println()

	writeTuples(ctx, "Creating folder hierarchy: alice owns folder:company, folder:company is parent of folder:product, diana views folder:company",
		fga.Tuple("user:alice", "owner", "folder:company"),
		fga.Tuple("folder:company", "parent", "folder:product"),
		fga.Tuple("user:diana", "viewer", "folder:company"),
	)

	check(ctx, "Can diana view folder:product?",
		"user:diana", "viewer", "folder:product",
		"diana is viewer on folder:company, which is parent of folder:product, 'viewer from parent' grants it",
	)

	check(ctx, "Can diana view document:roadmap?",
		"user:diana", "viewer", "document:roadmap",
		"two-hop inheritance: folder:company -> folder:product -> document:roadmap",
	)

	check(ctx, "Can diana edit document:roadmap?",
		"user:diana", "editor", "document:roadmap",
		"viewer does not imply editor -- diana can only view, not edit",
	)
}

func chapter5(ctx context.Context) {
	printHeader("Groups via Usersets")

	fmt.Println("  Concept: Instead of granting access user-by-user, you can grant access")
	fmt.Println("  to an entire group. Members of the group automatically get access.")
	fmt.Println("  Model: organization#member as viewer on a folder gives all members viewer access.")
	fmt.Println()

	writeTuples(ctx, "Adding eve and frank to organization:acme, granting acme#member viewer on folder:product",
		fga.Tuple("user:eve", "member", "organization:acme"),
		fga.Tuple("user:frank", "member", "organization:acme"),
		fga.Tuple("organization:acme#member", "viewer", "folder:product"),
	)

	check(ctx, "Can eve view document:roadmap?",
		"user:eve", "viewer", "document:roadmap",
		"eve is member of acme, acme#member is viewer on folder:product, folder:product is parent of document:roadmap",
	)

	check(ctx, "Can frank view document:roadmap?",
		"user:frank", "viewer", "document:roadmap",
		"frank is also a member of acme, same path as eve",
	)

	fmt.Println()
	writeTuples(ctx, "Adding grace to organization:acme (no new folder tuples needed!)",
		fga.Tuple("user:grace", "member", "organization:acme"),
	)

	check(ctx, "Can grace view document:roadmap?",
		"user:grace", "viewer", "document:roadmap",
		"grace automatically gets access via acme membership -- no per-user grants needed",
	)
}

func chapter6(ctx context.Context) {
	printHeader("Public Sharing (Wildcards)")

	fmt.Println("  Concept: Wildcard grants give access to everyone, like a public link.")
	fmt.Println("  user:* as viewer means any user can view the document.")
	fmt.Println()

	writeTuples(ctx, "Alice creates a public memo: alice owns it, everyone (user:*) can view it",
		fga.Tuple("user:alice", "owner", "document:public-memo"),
		fga.Tuple("user:*", "viewer", "document:public-memo"),
	)

	check(ctx, "Can randomstranger view document:public-memo?",
		"user:randomstranger", "viewer", "document:public-memo",
		"user:* is viewer, so any user (including randomstranger) can view",
	)

	check(ctx, "Can randomstranger edit document:public-memo?",
		"user:randomstranger", "editor", "document:public-memo",
		"wildcard only grants viewer, not editor -- randomstranger cannot edit",
	)
}

func chapter7(ctx context.Context) {
	printHeader("Reverse Queries (ListObjects)")

	fmt.Println("  Concept: Instead of asking 'can user X access object Y?', we ask")
	fmt.Println("  'which objects of type T can user X access?' -- the reverse query.")
	fmt.Println()

	listObjects(ctx, "Which documents can diana view?",
		"user:diana", "viewer", "document",
	)

	listObjects(ctx, "Which folders can eve view?",
		"user:eve", "viewer", "folder",
	)

	listObjects(ctx, "Which documents can alice view?",
		"user:alice", "viewer", "document",
	)
}

func chapter8(ctx context.Context) {
	printHeader("Expand (Resolution Tree)")

	fmt.Println("  Concept: Expand shows WHY a permission exists by returning the full")
	fmt.Println("  resolution tree -- all paths that grant the relationship.")
	fmt.Println()

	fmt.Println("  Expanding: viewer on document:roadmap")
	fmt.Println()

	tree, err := client.Expand(ctx, "viewer", "document:roadmap")
	if err != nil {
		fmt.Printf("  Error expanding: %v\n", err)
		return
	}
	if tree == nil {
		fmt.Println("  (no tree returned)")
		return
	}

	root := tree.GetRoot()
	printTree(&root, "  ", true)
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func printHeader(title string) {
	chapter++
	header := fmt.Sprintf("Chapter %d: %s", chapter, title)
	line := strings.Repeat("─", 66-len(header))
	fmt.Println()
	fmt.Printf("── %s %s\n", header, line)
	fmt.Println()
}

func writeTuples(ctx context.Context, description string, tuples ...fga.TupleKey) {
	fmt.Printf("  %s\n", description)
	fmt.Println("  Writing tuples:")
	for _, t := range tuples {
		fmt.Printf("    + %s#%s@%s\n", t.Object, t.Relation, t.User)
	}
	fmt.Println()

	if err := client.WriteTuples(ctx, tuples); err != nil {
		log.Fatalf("  Failed to write tuples: %v\n", err)
	}
}

func check(ctx context.Context, question, user, relation, object, because string) {
	allowed, err := client.Check(ctx, user, relation, object)
	if err != nil {
		fmt.Printf("  Q: %s\n", question)
		fmt.Printf("     Check(%s, %s, %s) -> ERROR: %v\n", user, relation, object, err)
		return
	}

	symbol := "DENIED"
	marker := "✗"
	if allowed {
		symbol = "ALLOWED"
		marker = "✓"
	}

	fmt.Printf("  Q: %s\n", question)
	fmt.Printf("     Check(%s, %s, %s) -> %s %s\n", user, relation, object, marker, symbol)
	fmt.Printf("     Because: %s\n", because)
	fmt.Println()
}

func listObjects(ctx context.Context, question, user, relation, objectType string) {
	objects, err := client.ListObjects(ctx, user, relation, objectType)
	if err != nil {
		fmt.Printf("  Q: %s\n", question)
		fmt.Printf("     ListObjects(%s, %s, %s) -> ERROR: %v\n", user, relation, objectType, err)
		return
	}

	fmt.Printf("  Q: %s\n", question)
	fmt.Printf("     ListObjects(%s, %s, %s) ->\n", user, relation, objectType)
	if len(objects) == 0 {
		fmt.Println("       (none)")
	}
	for _, obj := range objects {
		fmt.Printf("       - %s\n", obj)
	}
	fmt.Println()
}

func waitForEnter() {
	if !pauseFlag {
		return
	}
	fmt.Print("  Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	fmt.Println()
}

// printTree recursively prints an OpenFGA UsersetTree Node.
func printTree(node *openfga.Node, indent string, last bool) {
	if node == nil {
		return
	}

	connector := "├── "
	if last {
		connector = "└── "
	}

	name := node.GetName()
	if name == "" {
		name = "(root)"
	}

	// Check what kind of node this is
	if node.Leaf != nil {
		printLeaf(node, indent+connector)
		return
	}

	if node.Union != nil {
		fmt.Printf("%s%s%s [union]\n", indent, connector, name)
		union := node.GetUnion()
		children := union.Nodes
		childIndent := indent + "│   "
		if last {
			childIndent = indent + "    "
		}
		for i := range children {
			printTree(&children[i], childIndent, i == len(children)-1)
		}
		return
	}

	if node.Intersection != nil {
		fmt.Printf("%s%s%s [intersection]\n", indent, connector, name)
		intersection := node.GetIntersection()
		children := intersection.Nodes
		childIndent := indent + "│   "
		if last {
			childIndent = indent + "    "
		}
		for i := range children {
			printTree(&children[i], childIndent, i == len(children)-1)
		}
		return
	}

	if node.Difference != nil {
		fmt.Printf("%s%s%s [difference]\n", indent, connector, name)
		childIndent := indent + "│   "
		if last {
			childIndent = indent + "    "
		}
		base := node.GetDifference().Base
		subtract := node.GetDifference().Subtract
		printTree(&base, childIndent, false)
		printTree(&subtract, childIndent, true)
		return
	}

	// Fallback: just print the name
	fmt.Printf("%s%s%s\n", indent, connector, name)
}

func printLeaf(node *openfga.Node, prefix string) {
	name := node.GetName()
	leaf := node.GetLeaf()

	if leaf.Users != nil {
		users := leaf.Users.Users
		if len(users) == 0 {
			fmt.Printf("%s%s [leaf: no users]\n", prefix, name)
		} else {
			fmt.Printf("%s%s [leaf: users]\n", prefix, name)
			// Reuse prefix length for indentation
			pad := strings.Repeat(" ", len(prefix))
			for _, u := range users {
				fmt.Printf("%s  - %s\n", pad, u)
			}
		}
		return
	}

	if leaf.Computed != nil {
		fmt.Printf("%s%s [computed: %s]\n", prefix, name, leaf.Computed.Userset)
		return
	}

	if leaf.TupleToUserset != nil {
		ttu := leaf.GetTupleToUserset()
		computed := ttu.GetComputed()
		var computedStr string
		for i, c := range computed {
			if i > 0 {
				computedStr += ", "
			}
			computedStr += c.Userset
		}
		fmt.Printf("%s%s [tupleToUserset: tupleset=%s, computed=[%s]]\n",
			prefix, name, ttu.GetTupleset(), computedStr)
		return
	}

	fmt.Printf("%s%s [leaf]\n", prefix, name)
}
