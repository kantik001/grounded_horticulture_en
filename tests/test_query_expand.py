from rag.query_expand import expand_query


def test_expand_marssoniosis():
    """expand_query adds glossary synonyms for a disease term."""
    # Glossary keys remain multilingual; Russian query still expands via agro_glossary.json.
    out = expand_query("What is known about marssonina on apple?").lower()
    assert "marssonina" in out


def test_expand_no_change_without_match():
    """A query without glossary terms is returned unchanged."""
    q = "How to irrigate pear trees?"
    assert expand_query(q) == q


def test_expand_rootstock_synonyms():
    """Expansion keeps the original rootstock terms in the query."""
    out = expand_query("Liberty on knip").lower()
    assert "liberty" in out
    assert "knip" in out
